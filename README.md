# cc_bridge

OpenAI-compatible API bridge that routes requests to either the Claude Code CLI or a local Ollama instance, with SQLite usage tracking and a built-in dashboard.

## Features

- OpenAI-compatible REST API (`/v1/chat/completions`, `/v1/models`)
- Streaming support (SSE / `stream: true`)
- **Claude Code backend** — stateless and stateful (`X-Session-ID`) sessions via the `claude` CLI
- **Ollama backend** — route any request to a local Ollama instance using the `ollama/` model prefix
- **SQLite usage tracking** — every request is logged (model, provider, tokens, duration, errors)
- **Built-in dashboard** at `/dashboard` — dark-theme table, auto-refreshes every 30s
- **Usage JSON API** at `/v1/usage`
- Optional `Authorization: Bearer` auth key

## Requirements

- Go 1.22+
- [Claude Code CLI](https://claude.ai/code) installed and authenticated
- (Optional) [Ollama](https://ollama.com) running locally for local model routing

## Build

```bash
go mod tidy
go build -o claude-bridge .
```

## Run

```bash
# Minimal
./claude-bridge

# With all options
PORT=8080 \
CLAUDE_WORKDIR=/path/to/workspace \
CLAUDE_SKIP_PERMS=true \
OLLAMA_URL=http://localhost:11434 \
USAGE_DB_PATH=./usage.db \
./claude-bridge
```

## Environment Variables

| Variable             | Default                    | Description                                      |
|----------------------|----------------------------|--------------------------------------------------|
| `PORT`               | `8080`                     | Listening port                                   |
| `CLAUDE_BIN`         | (PATH lookup)              | Override claude binary path                      |
| `CLAUDE_WORKDIR`     | (empty)                    | Working directory for claude subprocess          |
| `CLAUDE_SKIP_PERMS`  | `false`                    | Pass `--dangerously-skip-permissions` to claude  |
| `CLAUDE_DEFAULT_MODEL` | `claude-code`            | Default model when none is specified             |
| `CLAUDE_AUTH_KEY`    | (empty)                    | Require `Authorization: Bearer <key>` if set     |
| `OLLAMA_URL`         | `http://localhost:11434`   | Ollama base URL                                  |
| `USAGE_DB_PATH`      | `./usage.db`               | SQLite database path (created automatically)     |

## Model Routing

| Model prefix   | Backend  | Example                     |
|----------------|----------|-----------------------------|
| `ollama/`      | Ollama   | `ollama/gemma4:e4b`         |
| `ollama/`      | Ollama   | `ollama/qwen3:8b`           |
| anything else  | Claude   | `claude-code`, `claude-sonnet-4-6` |

## API Endpoints

### Chat completions

```bash
# Claude Code (default)
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-code","messages":[{"role":"user","content":"Hello"}]}'

# Ollama local model
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"ollama/gemma4:e4b","messages":[{"role":"user","content":"Hello"}]}'
```

### Stateful sessions (Claude)

Pass `X-Session-ID: <uuid>` to maintain conversation context across requests. The bridge maps your client session ID to a Claude session ID using `--session-id` / `--resume`.

### Stateful sessions (Ollama)

Same `X-Session-ID` header. The bridge maintains conversation history in memory and injects it on each request (Ollama has no native session resume).

### Health check

```bash
curl http://localhost:8080/health
```

### Models list

```bash
curl http://localhost:8080/v1/models
```

### Usage dashboard

Open in browser: `http://localhost:8080/dashboard`

### Usage JSON

```bash
curl http://localhost:8080/v1/usage
```

Returns an array of usage summary rows grouped by model and provider:

```json
[
  {
    "model": "claude-code",
    "provider": "claude",
    "requests": 42,
    "errors": 1,
    "prompt_tokens": 18400,
    "completion_tokens": 6200,
    "total_tokens": 24600,
    "avg_duration_ms": 3200
  }
]
```

## Delegation Pattern (from an orchestrator agent)

```json
POST http://localhost:8080/v1/chat/completions
{
  "model": "claude-code",
  "messages": [
    {"role": "system", "content": "<briefing: user, project context, absolute paths>"},
    {"role": "user",   "content": "<concrete task>"}
  ],
  "stream": false
}
```

For multi-turn tasks, add `X-Session-ID: <uuid>` to maintain context across calls.
