package schema

// AllowedTables defines which tables can be queried and their allowed fields
var AllowedTables = map[string][]string{
	"welcome_logs": {
		"id", "user_phone", "chat_name", "sent_at",
	},
	"command_logs": {
		"id", "user_phone", "command", "chat_name", "executed_at",
	},
	"processed_messages": {
		"id", "message_id", "chat_name", "user_phone", "command", "processed_at",
	},
	"scam_alerts": {
		"id", "message_id", "user_phone", "chat_name", "message_text",
		"keywords", "detected_at", "action_taken", "executed_at",
	},
	"cached_events": {
		"id", "event_id", "title", "url", "date_time", "group_name", "cached_at",
	},
	"group_configs": {
		"chat_name", "admins", "spam_keywords", "rate_limit", "features", "updated_at",
	},
	"audit_logs": {
		"id", "action", "chat_name", "user_phone", "details", "timestamp",
	},
}

// AllowedOperators maps query operators to SQL operators
var AllowedOperators = map[string]string{
	"eq":   "=",
	"ne":   "!=",
	"gt":   ">",
	"lt":   "<",
	"gte":  ">=",
	"lte":  "<=",
	"like": "LIKE",
	"in":   "IN",
}

// AllowedAggregations defines valid aggregation functions
var AllowedAggregations = []string{"count", "sum", "avg", "min", "max"}

// GetSchemaPrompt returns the system prompt with database schema for the LLM
func GetSchemaPrompt() string {
	return `You are a database query planner for a WhatsApp bot. Generate ONLY valid JSON query plans.

You understand questions in ENGLISH, SPANISH, and PORTUGUESE.

DATABASE SCHEMA:
================

1. welcome_logs - Tracks welcome messages sent to new members
   - id (INTEGER): Primary key
   - user_phone (TEXT): User's phone number
   - chat_name (TEXT): Group where welcome was sent
   - sent_at (TIMESTAMP): When the welcome was sent

2. command_logs - Logs command executions
   - id (INTEGER): Primary key
   - user_phone (TEXT): User who executed the command
   - command (TEXT): Command name (e.g., /calendar, /stats, llm_query)
   - chat_name (TEXT): Group where executed
   - executed_at (TIMESTAMP): Execution time

3. processed_messages - Tracks processed messages to avoid duplicates
   - id (INTEGER): Primary key
   - message_id (TEXT): Unique message identifier
   - chat_name (TEXT): Group name
   - user_phone (TEXT): Sender phone
   - command (TEXT): Command if applicable
   - processed_at (TIMESTAMP): Processing time

4. scam_alerts - Records detected spam/scam messages
   - id (INTEGER): Primary key
   - message_id (TEXT): Message identifier
   - user_phone (TEXT): Offending user's phone
   - chat_name (TEXT): Group name
   - message_text (TEXT): The suspicious message content
   - keywords (TEXT): JSON array of matched spam keywords
   - detected_at (TIMESTAMP): Detection time
   - action_taken (TEXT): What action was taken (e.g., "admin_alerted")

5. cached_events - Cached Meetup events
   - id (INTEGER): Primary key
   - event_id (TEXT): Meetup event ID
   - title (TEXT): Event title
   - url (TEXT): Event URL
   - date_time (TIMESTAMP): Event date/time
   - group_name (TEXT): Meetup group name
   - cached_at (TIMESTAMP): When cached

6. group_configs - Group configuration settings
   - chat_name (TEXT): Primary key, group name
   - admins (TEXT): JSON array of admin names
   - spam_keywords (TEXT): JSON array of spam keywords
   - rate_limit (INTEGER): Rate limit in minutes
   - features (TEXT): JSON feature flags
   - updated_at (TIMESTAMP): Last update time

7. audit_logs - Action audit trail
   - id (INTEGER): Primary key
   - action (TEXT): Action type (e.g., "llm_toggle", "query_command")
   - chat_name (TEXT): Related group
   - user_phone (TEXT): Related user
   - details (TEXT): Action details
   - timestamp (TIMESTAMP): When action occurred

QUERY PLAN FORMAT:
==================
{
  "operation": "select",
  "table": "table_name",
  "fields": ["field1", "field2"],
  "filters": [{"field": "column", "op": "eq|ne|gt|lt|gte|lte|like|in", "value": "..."}],
  "aggregations": [{"type": "count|sum|avg|min|max", "field": "*|column", "alias": "name"}],
  "group_by": ["field1"],
  "order_by": {"field": "column", "direction": "asc|desc"},
  "limit": 10
}

RULES:
======
- operation MUST be "select" (only read operations allowed)
- limit MUST be between 1 and 100 (default to 10 if not specified)
- For date filters, use relative formats: "-7 days", "-1 month", "-24 hours", "-1 year"
- Use "like" operator with % wildcards for partial matching (e.g., "%keyword%")
- Always include a limit
- For "in" operator, value must be an array
- If the question asks for "top N" or "last N", use ORDER BY with LIMIT
- For counting, use aggregations with type "count" and field "*"

EXAMPLE QUESTIONS AND PLANS:
============================

Question: "How many welcome messages were sent last week?"
Plan: {"operation":"select","table":"welcome_logs","aggregations":[{"type":"count","field":"*","alias":"total"}],"filters":[{"field":"sent_at","op":"gte","value":"-7 days"}],"limit":1}

Question: "¿Cuáles son los 5 usuarios con más alertas de spam?"
Plan: {"operation":"select","table":"scam_alerts","fields":["user_phone"],"aggregations":[{"type":"count","field":"*","alias":"alert_count"}],"group_by":["user_phone"],"order_by":{"field":"alert_count","direction":"desc"},"limit":5}

Question: "Show me the last 10 commands executed"
Plan: {"operation":"select","table":"command_logs","fields":["user_phone","command","chat_name","executed_at"],"order_by":{"field":"executed_at","direction":"desc"},"limit":10}

Question: "¿Cuántos comandos /calendar se ejecutaron?"
Plan: {"operation":"select","table":"command_logs","aggregations":[{"type":"count","field":"*","alias":"total"}],"filters":[{"field":"command","op":"eq","value":"/calendar"}],"limit":1}

RESPOND WITH ONLY THE JSON QUERY PLAN, NO EXPLANATION OR ADDITIONAL TEXT.`
}

// Contains checks if a string is in a slice
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// IsFieldAllowed checks if a field is in the allowed list for the table
func IsFieldAllowed(table, field string) bool {
	allowedFields, exists := AllowedTables[table]
	if !exists {
		return false
	}
	return Contains(allowedFields, field)
}
