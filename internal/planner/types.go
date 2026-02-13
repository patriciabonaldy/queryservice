package planner

// QueryRequest represents the API request payload
type QueryRequest struct {
	Question string `json:"question"`
}

// QueryResponse represents the API response
type QueryResponse struct {
	Success   bool                     `json:"success"`
	Question  string                   `json:"question"`
	Language  string                   `json:"language"`
	QueryPlan *QueryPlan               `json:"query_plan,omitempty"`
	Data      []map[string]interface{} `json:"data,omitempty"`
	RowCount  int                      `json:"row_count"`
	Error     string                   `json:"error,omitempty"`
}

// QueryPlan represents the LLM-generated structured query plan
type QueryPlan struct {
	Operation    string        `json:"operation"`
	Table        string        `json:"table"`
	Fields       []string      `json:"fields,omitempty"`
	Filters      []QueryFilter `json:"filters,omitempty"`
	Aggregations []Aggregation `json:"aggregations,omitempty"`
	GroupBy      []string      `json:"group_by,omitempty"`
	OrderBy      *OrderBy      `json:"order_by,omitempty"`
	Limit        int           `json:"limit"`
}

// QueryFilter represents a WHERE condition
type QueryFilter struct {
	Field string      `json:"field"`
	Op    string      `json:"op"`
	Value interface{} `json:"value"`
}

// Aggregation represents an aggregate function
type Aggregation struct {
	Type  string `json:"type"`
	Field string `json:"field"`
	Alias string `json:"alias"`
}

// OrderBy represents ordering specification
type OrderBy struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

// LLMRequest represents the request payload to LLM Studio
type LLMRequest struct {
	Model       string       `json:"model"`
	Messages    []LLMMessage `json:"messages"`
	Temperature float64      `json:"temperature"`
	MaxTokens   int          `json:"max_tokens"`
	Stream      bool         `json:"stream"`
}

// LLMMessage represents a chat message for LLM
type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMResponse represents the response from LLM Studio
type LLMResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
