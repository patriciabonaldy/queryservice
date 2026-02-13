package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/patriciabonaldy/queryservice/internal/executor"
	"github.com/patriciabonaldy/queryservice/internal/planner"
	"github.com/patriciabonaldy/queryservice/internal/schema"
)

// Handler handles HTTP requests for the Query API
type Handler struct {
	planner  *planner.Planner
	executor *executor.Executor
}

// NewHandler creates a new API handler
func NewHandler(db *sql.DB, llmBaseURL, llmModel string) *Handler {
	return &Handler{
		planner:  planner.New(llmBaseURL, llmModel),
		executor: executor.New(db),
	}
}

// RegisterRoutes registers the API routes on the given ServeMux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/query", h.handleQuery)
	mux.HandleFunc("/api/health", h.handleHealth)
	mux.HandleFunc("/api/schema", h.handleSchema)
}

// handleQuery processes natural language queries
func (h *Handler) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, http.StatusMethodNotAllowed, "only POST method is allowed")
		return
	}

	var req planner.QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	if req.Question == "" {
		h.sendError(w, http.StatusBadRequest, "question is required")
		return
	}

	if len(req.Question) > 500 {
		h.sendError(w, http.StatusBadRequest, "question is too long (max 500 characters)")
		return
	}

	language := planner.DetectLanguage(req.Question)

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	plan, err := h.planner.GenerateQueryPlan(ctx, req.Question)
	if err != nil {
		log.Printf("Error generating query plan: %v", err)
		h.sendErrorResponse(w, req.Question, language, "failed to understand the question: "+err.Error())
		return
	}

	if err := h.planner.Validate(plan); err != nil {
		log.Printf("Query plan validation failed: %v", err)
		h.sendErrorResponse(w, req.Question, language, "invalid query: "+err.Error())
		return
	}

	data, err := h.executor.Execute(ctx, plan)
	if err != nil {
		log.Printf("Query execution error: %v", err)
		h.sendErrorResponse(w, req.Question, language, "query execution failed: "+err.Error())
		return
	}

	response := planner.QueryResponse{
		Success:   true,
		Question:  req.Question,
		Language:  language,
		QueryPlan: plan,
		Data:      data,
		RowCount:  len(data),
	}

	h.sendJSON(w, http.StatusOK, response)
}

// handleHealth returns the API health status
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendError(w, http.StatusMethodNotAllowed, "only GET method is allowed")
		return
	}

	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "query-service",
	}

	h.sendJSON(w, http.StatusOK, response)
}

// handleSchema returns the available database schema
func (h *Handler) handleSchema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendError(w, http.StatusMethodNotAllowed, "only GET method is allowed")
		return
	}

	tables := make(map[string]interface{})
	for table, fields := range schema.AllowedTables {
		tables[table] = map[string]interface{}{
			"fields": fields,
		}
	}

	response := map[string]interface{}{
		"tables":       tables,
		"operators":    []string{"eq", "ne", "gt", "lt", "gte", "lte", "like", "in"},
		"aggregations": schema.AllowedAggregations,
	}

	h.sendJSON(w, http.StatusOK, response)
}

func (h *Handler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

func (h *Handler) sendError(w http.ResponseWriter, status int, message string) {
	response := map[string]interface{}{
		"success": false,
		"error":   message,
	}
	h.sendJSON(w, status, response)
}

func (h *Handler) sendErrorResponse(w http.ResponseWriter, question, language, message string) {
	response := planner.QueryResponse{
		Success:  false,
		Question: question,
		Language: language,
		Error:    message,
	}
	h.sendJSON(w, http.StatusOK, response)
}
