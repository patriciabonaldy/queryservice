package planner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/patriciabonaldy/queryservice/internal/schema"
)

// Planner handles LLM interaction for query plan generation
type Planner struct {
	llmBaseURL string
	llmModel   string
	httpClient *http.Client
}

// New creates a new query planner
func New(llmBaseURL, llmModel string) *Planner {
	return &Planner{
		llmBaseURL: llmBaseURL,
		llmModel:   llmModel,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// GenerateQueryPlan asks the LLM to generate a query plan from natural language
func (p *Planner) GenerateQueryPlan(ctx context.Context, question string) (*QueryPlan, error) {
	systemPrompt := schema.GetSchemaPrompt()

	messages := []LLMMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: question,
		},
	}

	request := LLMRequest{
		Model:       p.llmModel,
		Messages:    messages,
		Temperature: 0.1,
		MaxTokens:   500,
		Stream:      false,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	log.Printf("Sending query plan request to LLM: %s", p.llmBaseURL)

	req, err := http.NewRequestWithContext(ctx, "POST", p.llmBaseURL+"/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request to LLM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LLM API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	var llmResp LLMResponse
	if err := json.Unmarshal(body, &llmResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling LLM response: %w", err)
	}

	if len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	content := llmResp.Choices[0].Message.Content
	content = extractJSON(content)

	log.Printf("LLM response: %s", content)

	var plan QueryPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, fmt.Errorf("error parsing query plan JSON: %w (content: %s)", err, content)
	}

	return &plan, nil
}

// extractJSON extracts JSON from the LLM response
func extractJSON(content string) string {
	content = strings.TrimSpace(content)

	// Remove markdown code blocks if present
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		if idx := strings.Index(content, "```"); idx != -1 {
			content = content[:idx]
		}
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		if idx := strings.Index(content, "```"); idx != -1 {
			content = content[:idx]
		}
	}

	// Find the first { and last }
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")

	if start != -1 && end != -1 && end > start {
		content = content[start : end+1]
	}

	return strings.TrimSpace(content)
}

// DetectLanguage detects the language of the question (en, es, pt)
func DetectLanguage(text string) string {
	normalizedText := normalizeText(text)

	spanishIndicators := []string{
		"cuantos", "cuantas", "cual", "cuales",
		"donde", "cuando", "quien", "quienes",
		"como", "porque", "ultimos", "ultima",
		"semana", "mes", "ano", "dia",
		"enviados", "enviaron", "detectados", "alertas",
		"usuarios", "mensajes", "comandos",
		"mostrar", "dame", "dime", "listar",
	}

	portugueseIndicators := []string{
		"quantos", "quantas", "qual", "quais",
		"onde", "quando", "quem", "como",
		"porque", "ultimos", "ultima",
		"semana", "mes", "ano", "dia",
		"enviados", "enviaram", "detectados", "alertas",
		"usuarios", "mensagens", "comandos",
		"mostrar", "mostra", "listar",
	}

	spanishScore := 0
	for _, indicator := range spanishIndicators {
		if strings.Contains(normalizedText, indicator) {
			spanishScore++
		}
	}

	portugueseScore := 0
	for _, indicator := range portugueseIndicators {
		if strings.Contains(normalizedText, indicator) {
			portugueseScore++
		}
	}

	if spanishScore > portugueseScore && spanishScore > 0 {
		return "es"
	}
	if portugueseScore > spanishScore && portugueseScore > 0 {
		return "pt"
	}

	return "en"
}

// normalizeText removes accents and converts to lowercase
func normalizeText(text string) string {
	text = strings.ToLower(text)

	replacements := map[rune]rune{
		'á': 'a', 'à': 'a', 'ã': 'a', 'â': 'a',
		'é': 'e', 'è': 'e', 'ê': 'e',
		'í': 'i', 'ì': 'i', 'î': 'i',
		'ó': 'o', 'ò': 'o', 'õ': 'o', 'ô': 'o',
		'ú': 'u', 'ù': 'u', 'û': 'u',
		'ñ': 'n', 'ç': 'c',
	}

	var result strings.Builder
	for _, r := range text {
		if replacement, ok := replacements[r]; ok {
			result.WriteRune(replacement)
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// Validate ensures the query plan is safe to execute
func (p *Planner) Validate(plan *QueryPlan) error {
	if strings.ToLower(plan.Operation) != "select" {
		return fmt.Errorf("only SELECT operations are allowed")
	}

	allowedFields, tableExists := schema.AllowedTables[plan.Table]
	if !tableExists {
		return fmt.Errorf("table '%s' is not queryable", plan.Table)
	}

	for _, field := range plan.Fields {
		if field != "*" && !schema.Contains(allowedFields, field) {
			return fmt.Errorf("field '%s' is not allowed for table '%s'", field, plan.Table)
		}
	}

	for _, filter := range plan.Filters {
		if !schema.Contains(allowedFields, filter.Field) {
			return fmt.Errorf("filter field '%s' is not allowed", filter.Field)
		}
		if _, ok := schema.AllowedOperators[filter.Op]; !ok {
			return fmt.Errorf("operator '%s' is not allowed", filter.Op)
		}
	}

	for _, agg := range plan.Aggregations {
		if !schema.Contains(schema.AllowedAggregations, agg.Type) {
			return fmt.Errorf("aggregation '%s' is not allowed", agg.Type)
		}
		if agg.Field != "*" && !schema.Contains(allowedFields, agg.Field) {
			return fmt.Errorf("aggregation field '%s' is not allowed", agg.Field)
		}
	}

	for _, field := range plan.GroupBy {
		if !schema.Contains(allowedFields, field) {
			return fmt.Errorf("group by field '%s' is not allowed", field)
		}
	}

	if plan.OrderBy != nil {
		isAggAlias := false
		for _, agg := range plan.Aggregations {
			if agg.Alias == plan.OrderBy.Field {
				isAggAlias = true
				break
			}
		}

		if !isAggAlias && !schema.Contains(allowedFields, plan.OrderBy.Field) {
			return fmt.Errorf("order by field '%s' is not allowed", plan.OrderBy.Field)
		}

		dir := strings.ToLower(plan.OrderBy.Direction)
		if dir != "asc" && dir != "desc" {
			return fmt.Errorf("invalid order direction '%s'", plan.OrderBy.Direction)
		}
	}

	if plan.Limit <= 0 || plan.Limit > 100 {
		plan.Limit = 10
	}

	return nil
}
