# claude-bridge

OpenAI-compatible local API bridge that routes requests to either the Claude Code CLI or a local Ollama instance, with SQLite usage tracking, Gorm persistence, Gin HTTP routing, in-memory sessions, and a built-in dashboard.

## Features

- OpenAI-compatible REST API:
  - `/v1/chat/completions`
  - `/v1/models`
- Streaming support using Server-Sent Events:
  - `stream: true`
- **Claude Code backend**
  - Stateless requests
  - Stateful sessions using `X-Session-ID`
  - Uses the local `claude` CLI
- **Ollama backend**
  - Routes requests to a local Ollama instance
  - Uses the `ollama/` model prefix
- **SQLite usage tracking**
  - Powered by Gorm
  - Logs model, provider, tokens, duration, and errors
- **Built-in dashboard**
  - Available at `/dashboard`
  - Auto-refreshes every 30 seconds
- **Usage JSON API**
  - Summary: `/v1/usage`
  - Recent records: `/v1/usage/recent`
- Optional local `Authorization: Bearer` auth key
- Designed as a local developer tool

## Requirements

- Go 1.25+
- [Claude Code CLI](https://claude.ai/code) installed and authenticated
- Optional: [Ollama](https://ollama.com) running locally
- Optional: Docker

## Project Structure

```txt
claude-bridge/
├─ cmd/
│  └─ claude-bridge/
│     └─ main.go
│
├─ internal/
│  ├─ config/
│  ├─ domain/
│  ├─ http/
│  │  ├─ handlers/
│  │  ├─ middleware/
│  │  └─ router.go
│  ├─ providers/
│  │  ├─ claude/
│  │  └─ ollama/
│  ├─ services/
│  ├─ sessions/
│  └─ storage/
│     ├─ models/
│     └─ repository/
│
├─ web/
│  └─ dashboard/
│     └─ template.go
│
├─ Dockerfile
├─ .env.example
├─ go.mod
└─ README.md
```

## Installation

```bash
go mod tidy
```

## Build

```bash
go build -o claude-bridge ./cmd/claude-bridge
```

On Windows:

```bash
go build -o claude-bridge.exe ./cmd/claude-bridge
```

## Run Locally

```bash
./claude-bridge
```

On Windows:

```powershell
.\claude-bridge.exe
```

With environment variables:

```bash
HOST=127.0.0.1 \
PORT=8080 \
CLAUDE_WORKDIR=/path/to/workspace \
CLAUDE_SKIP_PERMS=false \
OLLAMA_URL=http://localhost:11434 \
USAGE_DB_PATH=./usage.db \
./claude-bridge
```

On Windows PowerShell:

```powershell
$env:HOST="127.0.0.1"
$env:PORT="8080"
$env:CLAUDE_WORKDIR="C:\Users\your-user\Development\my-project"
$env:CLAUDE_SKIP_PERMS="false"
$env:OLLAMA_URL="http://localhost:11434"
$env:USAGE_DB_PATH="./usage.db"

.\claude-bridge.exe
```

## Docker

The project includes a `Dockerfile` for running `claude-bridge` in a container.

This is mainly useful when using the Ollama backend. For Claude Code, running directly on the host is usually simpler because the container would need access to the `claude` CLI and its authenticated credentials.

### Build Docker Image

```bash
docker build -t claude-bridge .
```

### Run Docker Container

```bash
docker run --rm -it \
  --name claude-bridge \
  -p 8080:8080 \
  -v claude_bridge_data:/data \
  claude-bridge
```

Then open:

```txt
http://127.0.0.1:8080/dashboard
```

Health check:

```bash
curl http://127.0.0.1:8080/health
```

### Run Docker with Environment Variables

Inside Docker, `HOST` should be `0.0.0.0` so the published port is reachable from the host.

```bash
docker run --rm -it \
  --name claude-bridge \
  -e HOST=0.0.0.0 \
  -e PORT=8080 \
  -e USAGE_DB_PATH=/data/usage.db \
  -e OLLAMA_URL=http://host.docker.internal:11434 \
  -p 8080:8080 \
  -v claude_bridge_data:/data \
  claude-bridge
```

### Docker with Ollama on Host

On Docker Desktop for Windows or macOS, use:

```env
OLLAMA_URL=http://host.docker.internal:11434
```

Test an Ollama model:

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "ollama/llama3.2",
    "messages": [
      {
        "role": "user",
        "content": "Hello"
      }
    ]
  }'
```

On Linux, if `host.docker.internal` is not available, run the container with:

```bash
docker run --rm -it \
  --name claude-bridge \
  --add-host=host.docker.internal:host-gateway \
  -p 8080:8080 \
  -v claude_bridge_data:/data \
  claude-bridge
```

### Docker with Local Auth

```bash
docker run --rm -it \
  --name claude-bridge \
  -e CLAUDE_BRIDGE_AUTH_KEY=your-local-secret \
  -p 8080:8080 \
  -v claude_bridge_data:/data \
  claude-bridge
```

Then requests must include:

```bash
-H "Authorization: Bearer your-local-secret"
```

Example:

```bash
curl http://127.0.0.1:8080/v1/models \
  -H "Authorization: Bearer your-local-secret"
```

### Docker Notes for Claude Code

The Docker image does not install or authenticate the Claude Code CLI by default.

For the Claude Code backend to work inside Docker, the container needs:

- The `claude` binary installed inside the image.
- Claude Code credentials available inside the container.
- A mounted workspace if you want Claude Code to operate on local files.
- Correct `CLAUDE_BIN` and `CLAUDE_WORKDIR` values.

For most local development workflows, run `claude-bridge` directly on the host when using Claude Code.

## Environment Variables

| Variable | Default | Description |
|---|---:|---|
| `HOST` | `127.0.0.1` locally, `0.0.0.0` in Docker | Listening host. |
| `PORT` | `8080` | HTTP server port. |
| `CLAUDE_BRIDGE_AUTH_KEY` | empty | Optional local auth key. If set, requests must include `Authorization: Bearer <key>`. |
| `CLAUDE_BIN` | PATH lookup | Override Claude CLI binary path. |
| `CLAUDE_WORKDIR` | empty | Working directory for the Claude subprocess. |
| `CLAUDE_SKIP_PERMS` | `false` | Pass `--dangerously-skip-permissions` to Claude. Use with caution. |
| `CLAUDE_DEFAULT_MODEL` | `claude-code` | Default model when none is specified. |
| `OLLAMA_URL` | `http://localhost:11434` locally, `http://host.docker.internal:11434` in Docker | Ollama base URL. |
| `USAGE_DB_PATH` | `./usage.db` locally, `/data/usage.db` in Docker | SQLite database path. Created automatically. |

## `.env.example`

```env
HOST=127.0.0.1
PORT=8080

CLAUDE_BRIDGE_AUTH_KEY=

CLAUDE_BIN=
CLAUDE_WORKDIR=
CLAUDE_DEFAULT_MODEL=claude-code
CLAUDE_SKIP_PERMS=false

OLLAMA_URL=http://localhost:11434

USAGE_DB_PATH=./usage.db
```

For Docker, use:

```env
HOST=0.0.0.0
PORT=8080
OLLAMA_URL=http://host.docker.internal:11434
USAGE_DB_PATH=/data/usage.db
```

## Model Routing

| Model prefix | Backend | Example |
|---|---|---|
| `ollama/` | Ollama | `ollama/llama3.2` |
| `ollama/` | Ollama | `ollama/qwen3:8b` |
| Anything else | Claude Code CLI | `claude-code`, `claude-sonnet-4-6` |

## MCP Support

`claude-bridge` can optionally enable MCP tools for the Claude Code backend.

Claude Code already supports MCP, so `claude-bridge` only forwards the correct flags to the `claude` CLI:

```txt
--mcp-config
--strict-mcp-config
--allowedTools
```

MCP is only supported for Claude Code requests. Ollama requests ignore MCP fields.

### Environment Variables

| Variable | Default | Description |
|---|---:|---|
| `MCP_CONFIG_PATH` | empty | Path to a local MCP registry JSON file. |
| `MCP_ALWAYS` | `false` | If true, load the MCP registry on every Claude request. |

### MCP Registry

Create a local MCP registry file based on `mcp.example.json`.

Example:

```json
{
  "mcpServers": {
    "projecthub": {
      "command": "npx",
      "args": ["-y", "projecthub-mcp"],
      "env": {
        "API_KEY": "REPLACE_ME"
      }
    }
  }
}
```

Then set:

```env
MCP_CONFIG_PATH=./mcp.json
```

Do not commit real MCP credentials.

### Per-request MCP

You can enable MCP tools per request using these fields:

| Field | Type | Description |
|---|---|---|
| `allowed_tools` | `string[]` | Tool allowlist passed to Claude Code. |
| `mcp_servers` | `string[]` | MCP servers to load from `MCP_CONFIG_PATH`. |
| `mcp_config` | `object` | Inline MCP config. Overrides the registry for that request. |
| `workdir` | `string` | Working directory for the Claude subprocess. |

Example:

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-code",
    "messages": [
      {
        "role": "user",
        "content": "Use the project MCP tools for this task."
      }
    ],
    "mcp_servers": ["projecthub"],
    "allowed_tools": ["mcp__projecthub"],
    "workdir": "/path/to/project"
  }'
```

Tool names follow Claude Code MCP naming:

```txt
mcp__<server>__<tool>
```

You can also allow all tools from a server with:

```txt
mcp__<server>
```

## API Endpoints

### Chat Completions

#### Claude Code

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-code",
    "messages": [
      {
        "role": "user",
        "content": "Hello"
      }
    ]
  }'
```

#### Ollama

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "ollama/llama3.2",
    "messages": [
      {
        "role": "user",
        "content": "Hello"
      }
    ]
  }'
```

### Streaming

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-code",
    "stream": true,
    "messages": [
      {
        "role": "user",
        "content": "Write a short paragraph about Go."
      }
    ]
  }'
```

With Ollama:

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "ollama/llama3.2",
    "stream": true,
    "messages": [
      {
        "role": "user",
        "content": "Write a short paragraph about Go."
      }
    ]
  }'
```

## Local Auth

Auth is optional.

If `CLAUDE_BRIDGE_AUTH_KEY` is empty, no authorization header is required.

If `CLAUDE_BRIDGE_AUTH_KEY` is set, every protected request must include:

```bash
-H "Authorization: Bearer your-local-secret"
```

Example:

```bash
curl http://127.0.0.1:8080/v1/models \
  -H "Authorization: Bearer your-local-secret"
```

## Stateful Sessions

### Claude Code Sessions

Pass `X-Session-ID` to keep context across requests.

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Session-ID: demo-session-1" \
  -d '{
    "model": "claude-code",
    "messages": [
      {
        "role": "user",
        "content": "Remember that my project is written in Go."
      }
    ]
  }'
```

The bridge maps your client session ID to a Claude session ID and uses Claude Code session resume internally.

### Ollama Sessions

Ollama does not provide native session resume in the same way, so `claude-bridge` keeps the conversation history in memory and injects it into each request.

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Session-ID: local-ollama-session-1" \
  -d '{
    "model": "ollama/llama3.2",
    "messages": [
      {
        "role": "user",
        "content": "Remember that I like concise answers."
      }
    ]
  }'
```

## Health Check

```bash
curl http://127.0.0.1:8080/health
```

Example response:

```json
{
  "status": "ok",
  "local": true,
  "host": "127.0.0.1",
  "port": "8080",
  "auth": false,
  "ollama_url": "http://localhost:11434",
  "usage_db": "./usage.db",
  "skip_perms": false
}
```

## Models List

```bash
curl http://127.0.0.1:8080/v1/models
```

## Usage Dashboard

Open in browser:

```txt
http://127.0.0.1:8080/dashboard
```

The dashboard shows usage grouped by model and provider.

It auto-refreshes every 30 seconds.

## Usage JSON

### Summary

```bash
curl http://127.0.0.1:8080/v1/usage
```

Example response:

```json
[
  {
    "model": "claude-code",
    "provider": "claude",
    "total_requests": 42,
    "errors": 1,
    "prompt_tokens": 18400,
    "completion_tokens": 6200,
    "total_tokens": 24600,
    "avg_duration_ms": 3200
  }
]
```

### Recent Records

```bash
curl http://127.0.0.1:8080/v1/usage/recent
```

With custom limit:

```bash
curl "http://127.0.0.1:8080/v1/usage/recent?limit=100"
```

## Delegation Pattern

`claude-bridge` can be used by an orchestrator agent or any OpenAI-compatible client.

```json
{
  "model": "claude-code",
  "messages": [
    {
      "role": "system",
      "content": "<briefing: user, project context, absolute paths>"
    },
    {
      "role": "user",
      "content": "<concrete task>"
    }
  ],
  "stream": false
}
```

For multi-turn tasks, add:

```txt
X-Session-ID: <your-session-id>
```

## Safety Notes

`claude-bridge` is intended to run locally.

By default, when running locally, it binds to:

```txt
127.0.0.1:8080
```

When running in Docker, it binds to:

```txt
0.0.0.0:8080
```

Avoid exposing it publicly unless you add stronger authentication, rate limiting, request limits, and sandboxing.

Be careful with:

```env
CLAUDE_SKIP_PERMS=true
```

This passes `--dangerously-skip-permissions` to Claude Code and should only be used in trusted local environments.

## Development

Run:

```bash
go run ./cmd/claude-bridge
```

Run tests:

```bash
go test ./...
```

Build:

```bash
go build -o claude-bridge ./cmd/claude-bridge
```

Build Docker image:

```bash
docker build -t claude-bridge .
```

## License

MIT
