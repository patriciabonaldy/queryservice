# Query Service

A natural language database query API for a WhatsApp bot. It converts questions in English, Spanish, and Portuguese into SQL queries using a local LLM, then executes them against a SQLite database.

## How it works

1. User sends a natural language question via the API
2. The question is enriched with hints (command names, date boundaries)
3. An LLM generates a structured JSON query plan
4. The query plan is validated against an allowlist of tables, fields, and operators
5. A parameterized SQL query is built and executed (read-only)
6. Results are returned as JSON

## Prerequisites

- Go 1.24+
- SQLite3
- A local LLM server compatible with the OpenAI `/v1/chat/completions` API (e.g., [LM Studio](https://lmstudio.ai/))

## Setup

1. Clone the repository:

```bash
git clone https://github.com/patriciabonaldy/queryservice.git
cd queryservice
```

2. Copy the example environment file and configure it:

```bash
cp example.env .env
```

3. Edit `.env` with your settings:

```env
QUERY_API_HOST=0.0.0.0       # Use 127.0.0.1 for local only
QUERY_API_PORT=8081
DB_PATH=./whatsapp_bot.db
LLM_BASE_URL=http://localhost:1234
LLM_MODEL=google/gemma-4-e2b
```

4. Build and run:

```bash
go build -o queryservice ./cmd/server
./queryservice
```

## API Endpoints

### POST /api/query

Execute a natural language query.

**Request:**

```bash
curl -X POST http://localhost:8081/api/query \
  -H "Content-Type: application/json" \
  -d '{"question": "how many people joined this week"}'
```

**Response:**

```json
{
  "success": true,
  "question": "how many people joined this week",
  "language": "en",
  "query_plan": {
    "operation": "select",
    "table": "welcome_logs",
    "filters": [{"field": "sent_at", "op": "gte", "value": "2026-04-13"}],
    "aggregations": [{"type": "count", "field": "*", "alias": "total"}],
    "limit": 1
  },
  "data": [{"total": 7}],
  "row_count": 1
}
```

### GET /api/health

Health check.

```bash
curl http://localhost:8081/api/health
```

### GET /api/schema

View available tables, fields, operators, and aggregations.

```bash
curl http://localhost:8081/api/schema
```

## Available Tables

| Table | Description |
|-------|-------------|
| `welcome_logs` | New members who joined/were welcomed |
| `command_logs` | Bot command executions (/calendar, /review, etc.) |
| `processed_messages` | Message deduplication tracking |
| `scam_alerts` | Detected spam/scam messages |
| `walk_reviews` | Walking event reviews |
| `cached_events` | Cached Meetup events |
| `group_configs` | Group settings |
| `audit_logs` | Action audit trail |

## Security

- Database is opened in **read-only** mode
- Only `SELECT` operations are allowed
- Tables and fields are validated against an allowlist
- SQL queries use parameterized values to prevent injection
- Input length is capped (500 chars for questions, 1000 for filter values)
- SQL injection patterns are detected and blocked via regex

## Project Structure

```
queryservice/
├── cmd/server/main.go         # HTTP server, middleware, graceful shutdown
├── internal/
│   ├── api/handler.go         # Request handlers
│   ├── config/config.go       # Environment-based configuration
│   ├── executor/executor.go   # SQL query builder and executor
│   ├── planner/
│   │   ├── planner.go         # LLM integration and question enrichment
│   │   └── types.go           # Data structures
│   └── schema/schema.go       # Table definitions and LLM prompt
├── example.env
└── .gitignore
```

## License

MIT
