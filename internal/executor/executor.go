package executor

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/patriciabonaldy/queryservice/internal/planner"
	"github.com/patriciabonaldy/queryservice/internal/schema"
)

// SQL Injection patterns to detect and reject
var sqlInjectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(union\s+select)`),
	regexp.MustCompile(`(?i)(;\s*drop\s+)`),
	regexp.MustCompile(`(?i)(;\s*delete\s+)`),
	regexp.MustCompile(`(?i)(;\s*update\s+)`),
	regexp.MustCompile(`(?i)(;\s*insert\s+)`),
	regexp.MustCompile(`(?i)(;\s*alter\s+)`),
	regexp.MustCompile(`(?i)(;\s*create\s+)`),
	regexp.MustCompile(`(?i)(;\s*truncate\s+)`),
	regexp.MustCompile(`(?i)(--)`),
	regexp.MustCompile(`(?i)(/\*)`),
	regexp.MustCompile(`(?i)(xp_)`),
	regexp.MustCompile(`(?i)(exec\s*\()`),
	regexp.MustCompile(`(?i)(execute\s*\()`),
	regexp.MustCompile(`(?i)(into\s+outfile)`),
	regexp.MustCompile(`(?i)(load_file)`),
	regexp.MustCompile(`(?i)(benchmark\s*\()`),
	regexp.MustCompile(`(?i)(sleep\s*\()`),
	regexp.MustCompile(`(?i)(0x[0-9a-f]+)`),
}

// Executor handles query plan execution with SQL injection protection
type Executor struct {
	db *sql.DB
}

// New creates a new query executor
func New(db *sql.DB) *Executor {
	return &Executor{db: db}
}

// Execute converts a query plan to SQL and executes it safely
func (e *Executor) Execute(ctx context.Context, plan *planner.QueryPlan) ([]map[string]interface{}, error) {
	if err := e.validateAgainstInjection(plan); err != nil {
		return nil, fmt.Errorf("security validation failed: %w", err)
	}

	sqlQuery, args, err := e.buildSQL(plan)
	if err != nil {
		return nil, fmt.Errorf("SQL build error: %w", err)
	}

	log.Printf("Executing safe query: %s with %d args", sqlQuery, len(args))

	rows, err := e.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query execution error: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("error getting columns: %w", err)
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

// validateAgainstInjection checks all string values in the plan for SQL injection patterns
func (e *Executor) validateAgainstInjection(plan *planner.QueryPlan) error {
	if err := validateIdentifier(plan.Table, "table"); err != nil {
		return err
	}

	if _, exists := schema.AllowedTables[plan.Table]; !exists {
		return fmt.Errorf("table '%s' is not allowed", plan.Table)
	}

	for _, field := range plan.Fields {
		if field != "*" {
			if err := validateIdentifier(field, "field"); err != nil {
				return err
			}
			if !schema.IsFieldAllowed(plan.Table, field) {
				return fmt.Errorf("field '%s' is not allowed for table '%s'", field, plan.Table)
			}
		}
	}

	for _, filter := range plan.Filters {
		if err := validateIdentifier(filter.Field, "filter field"); err != nil {
			return err
		}
		if !schema.IsFieldAllowed(plan.Table, filter.Field) {
			return fmt.Errorf("filter field '%s' is not allowed", filter.Field)
		}
		if err := validateFilterValue(filter.Value); err != nil {
			return fmt.Errorf("invalid filter value: %w", err)
		}
	}

	for _, agg := range plan.Aggregations {
		if agg.Field != "*" {
			if err := validateIdentifier(agg.Field, "aggregation field"); err != nil {
				return err
			}
			if !schema.IsFieldAllowed(plan.Table, agg.Field) {
				return fmt.Errorf("aggregation field '%s' is not allowed", agg.Field)
			}
		}
		if agg.Alias != "" {
			if err := validateIdentifier(agg.Alias, "alias"); err != nil {
				return err
			}
		}
	}

	for _, field := range plan.GroupBy {
		if err := validateIdentifier(field, "group by field"); err != nil {
			return err
		}
		if !schema.IsFieldAllowed(plan.Table, field) {
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

		if err := validateIdentifier(plan.OrderBy.Field, "order by field"); err != nil {
			return err
		}

		if !isAggAlias && !schema.IsFieldAllowed(plan.Table, plan.OrderBy.Field) {
			return fmt.Errorf("order by field '%s' is not allowed", plan.OrderBy.Field)
		}

		direction := strings.ToLower(plan.OrderBy.Direction)
		if direction != "asc" && direction != "desc" {
			return fmt.Errorf("invalid order direction: %s", plan.OrderBy.Direction)
		}
	}

	return nil
}

// validateIdentifier ensures an identifier (table, field name) is safe
func validateIdentifier(name, context string) error {
	if name == "" {
		return fmt.Errorf("%s cannot be empty", context)
	}

	if !isLetter(rune(name[0])) {
		return fmt.Errorf("%s '%s' must start with a letter", context, name)
	}

	for _, r := range name {
		if !isLetter(r) && !isDigit(r) && r != '_' {
			return fmt.Errorf("%s '%s' contains invalid character '%c'", context, name, r)
		}
	}

	if len(name) > 64 {
		return fmt.Errorf("%s '%s' is too long (max 64 characters)", context, name)
	}

	if containsSQLInjection(name) {
		return fmt.Errorf("%s '%s' contains potentially malicious content", context, name)
	}

	return nil
}

// validateFilterValue checks if a filter value is safe
func validateFilterValue(value interface{}) error {
	switch v := value.(type) {
	case string:
		if containsSQLInjection(v) {
			return fmt.Errorf("value contains potentially malicious content")
		}
		if len(v) > 1000 {
			return fmt.Errorf("value is too long (max 1000 characters)")
		}
	case []interface{}:
		for _, item := range v {
			if err := validateFilterValue(item); err != nil {
				return err
			}
		}
	case []string:
		for _, item := range v {
			if containsSQLInjection(item) {
				return fmt.Errorf("array value contains potentially malicious content")
			}
		}
	case int, int64, float64, bool, nil:
		// Safe types
	default:
		return fmt.Errorf("unsupported value type: %T", value)
	}
	return nil
}

// containsSQLInjection checks if a string contains SQL injection patterns
func containsSQLInjection(s string) bool {
	for _, pattern := range sqlInjectionPatterns {
		if pattern.MatchString(s) {
			log.Printf("SQL injection pattern detected: %s", s)
			return true
		}
	}
	return false
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// buildSQL constructs a parameterized SQL query from the plan
func (e *Executor) buildSQL(plan *planner.QueryPlan) (string, []interface{}, error) {
	var args []interface{}

	var selectParts []string

	for _, field := range plan.Fields {
		if field == "*" {
			selectParts = append(selectParts, "*")
		} else {
			selectParts = append(selectParts, field)
		}
	}

	for _, agg := range plan.Aggregations {
		aggType := strings.ToUpper(agg.Type)
		var aggField string
		if agg.Field == "*" {
			aggField = "*"
		} else {
			aggField = agg.Field
		}

		aggSQL := fmt.Sprintf("%s(%s)", aggType, aggField)
		if agg.Alias != "" {
			aggSQL += " AS " + agg.Alias
		}
		selectParts = append(selectParts, aggSQL)
	}

	if len(selectParts) == 0 {
		selectParts = []string{"*"}
	}

	sqlQuery := fmt.Sprintf("SELECT %s FROM %s", strings.Join(selectParts, ", "), plan.Table)

	if len(plan.Filters) > 0 {
		var whereParts []string
		for _, filter := range plan.Filters {
			op := schema.AllowedOperators[filter.Op]

			if filter.Op == "in" {
				values := toInterfaceSlice(filter.Value)
				if len(values) == 0 {
					continue
				}
				placeholders := make([]string, len(values))
				for i, v := range values {
					placeholders[i] = "?"
					args = append(args, v)
				}
				whereParts = append(whereParts, fmt.Sprintf("%s IN (%s)",
					filter.Field, strings.Join(placeholders, ", ")))
			} else if filter.Op == "like" {
				whereParts = append(whereParts, fmt.Sprintf("%s LIKE ?", filter.Field))
				args = append(args, filter.Value)
			} else {
				whereParts = append(whereParts, fmt.Sprintf("%s %s ?", filter.Field, op))
				args = append(args, convertDateValue(filter.Value))
			}
		}
		if len(whereParts) > 0 {
			sqlQuery += " WHERE " + strings.Join(whereParts, " AND ")
		}
	}

	if len(plan.GroupBy) > 0 {
		sqlQuery += " GROUP BY " + strings.Join(plan.GroupBy, ", ")
	}

	if plan.OrderBy != nil {
		direction := strings.ToUpper(plan.OrderBy.Direction)
		if direction != "ASC" && direction != "DESC" {
			direction = "ASC"
		}
		sqlQuery += fmt.Sprintf(" ORDER BY %s %s", plan.OrderBy.Field, direction)
	}

	limit := plan.Limit
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	sqlQuery += fmt.Sprintf(" LIMIT %d", limit)

	return sqlQuery, args, nil
}

func toInterfaceSlice(value interface{}) []interface{} {
	switch v := value.(type) {
	case []interface{}:
		return v
	case []string:
		result := make([]interface{}, len(v))
		for i, s := range v {
			result[i] = s
		}
		return result
	default:
		return nil
	}
}

func convertDateValue(value interface{}) interface{} {
	strVal, ok := value.(string)
	if !ok {
		return value
	}

	relativePattern := regexp.MustCompile(`^-(\d+)\s*(day|days|hour|hours|week|weeks|month|months|year|years)$`)
	matches := relativePattern.FindStringSubmatch(strings.ToLower(strVal))

	if len(matches) == 3 {
		amount, err := strconv.Atoi(matches[1])
		if err != nil {
			return strVal
		}

		unit := matches[2]
		var duration time.Duration

		switch unit {
		case "hour", "hours":
			duration = time.Duration(amount) * time.Hour
		case "day", "days":
			duration = time.Duration(amount) * 24 * time.Hour
		case "week", "weeks":
			duration = time.Duration(amount) * 7 * 24 * time.Hour
		case "month", "months":
			duration = time.Duration(amount) * 30 * 24 * time.Hour
		case "year", "years":
			duration = time.Duration(amount) * 365 * 24 * time.Hour
		default:
			return strVal
		}

		return time.Now().Add(-duration).Format("2006-01-02 15:04:05")
	}

	if strVal == "now" {
		return time.Now().Format("2006-01-02 15:04:05")
	}

	return strVal
}
