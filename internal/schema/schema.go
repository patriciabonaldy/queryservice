package schema

import (
	"fmt"
	"time"
)

// AllowedTables defines which tables can be queried and their allowed fields
var AllowedTables = map[string][]string{
	"welcome_logs": {
		"id", "user_phone", "chat_name", "sent_at", "message_id",
	},
	"command_logs": {
		"id", "user_phone", "command", "chat_name", "executed_at",
	},
	"processed_messages": {
		"id", "message_id", "chat_name", "user_phone", "command", "processed_at",
	},
	"scam_alerts": {
		"id", "message_id", "user_phone", "chat_name", "message_text",
		"keywords", "detected_at", "action_taken",
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
	"walk_reviews": {
		"id", "walk_name", "walk_date", "route_description", "places_visited",
		"cafe_restaurant", "places_to_eat_nearby", "transportation",
		"path_conditions", "additional_notes", "organizer_name",
		"contributors", "chat_name", "created_at", "updated_at",
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
	now := time.Now()
	today := now.Format("2006-01-02")
	weekday := now.Weekday()
	// Monday=1 .. Sunday=7; time.Weekday: Sunday=0, Monday=1 .. Saturday=6
	daysSinceMonday := int(weekday) - 1
	if daysSinceMonday < 0 {
		daysSinceMonday = 6 // Sunday
	}
	mondayDate := now.AddDate(0, 0, -daysSinceMonday).Format("2006-01-02")

	return fmt.Sprintf(`You are a JSON query planner. Output ONLY a single JSON object, no text, no explanation, no thinking.

TODAY: %s (%s). This week started on Monday %s.

TABLES (name — purpose: columns):
welcome_logs — new members who joined/were welcomed: id, user_phone, chat_name, sent_at, message_id
command_logs — bot command executions (/calendar, /review, etc): id, user_phone, command, chat_name, executed_at
processed_messages — message deduplication tracking: id, message_id, chat_name, user_phone, command, processed_at
scam_alerts — detected spam/scam messages: id, message_id, user_phone, chat_name, message_text, keywords, detected_at, action_taken
walk_reviews — walking event reviews: id, walk_name, walk_date, route_description, places_visited, cafe_restaurant, places_to_eat_nearby, transportation, path_conditions, additional_notes, organizer_name, contributors, chat_name, created_at, updated_at
cached_events — cached Meetup events: id, event_id, title, url, date_time, group_name, cached_at
group_configs — group settings: chat_name, admins, spam_keywords, rate_limit, features, updated_at
audit_logs — action audit trail: id, action, chat_name, user_phone, details, timestamp

FORMAT: {"operation":"select","table":"...","fields":["..."],"filters":[{"field":"...","op":"eq|ne|gt|lt|gte|lte|like|in","value":"..."}],"aggregations":[{"type":"count|sum|avg|min|max","field":"...","alias":"..."}],"group_by":["..."],"order_by":{"field":"...","direction":"asc|desc"},"limit":10}

RULES: operation must be "select". limit 1-100 (default 10). Date filters: use absolute dates (e.g. "%s") when the user says "this week/month", use relative formats ("-7 days", "-1 month", "-24 hours") for "last week/month". "This week" means from Monday of the current week. Use "like" with %% for partial match. For "in", value is an array. For counting use aggregations with count/*.

EXAMPLES:
Q: "How many welcome messages were sent last week?"
A: {"operation":"select","table":"welcome_logs","aggregations":[{"type":"count","field":"*","alias":"total"}],"filters":[{"field":"sent_at","op":"gte","value":"-7 days"}],"limit":1}

Q: "How many times was /calendar executed this week?"
A: {"operation":"select","table":"command_logs","aggregations":[{"type":"count","field":"*","alias":"total"}],"filters":[{"field":"command","op":"eq","value":"/calendar"},{"field":"executed_at","op":"gte","value":"%s"}],"limit":1}`, today, weekday.String(), mondayDate, mondayDate, mondayDate) + `

Q: "Who used /review last month?"
A: {"operation":"select","table":"command_logs","fields":["user_phone","chat_name","executed_at"],"filters":[{"field":"command","op":"eq","value":"/review"},{"field":"executed_at","op":"gte","value":"-1 month"}],"order_by":{"field":"executed_at","direction":"desc"},"limit":10}

Q: "Show me the last 10 commands executed"
A: {"operation":"select","table":"command_logs","fields":["user_phone","command","chat_name","executed_at"],"order_by":{"field":"executed_at","direction":"desc"},"limit":10}`
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
