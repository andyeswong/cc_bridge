# cc_bridge

OpenAI-compatible API bridge for Claude Code CLI.

Exposes `/v1/chat/completions` and `/v1/models` endpoints backed by the `claude` CLI.

## Features
- OpenAI-compatible REST API (drop-in for any OpenAI client)
- Streaming support (SSE / `stream: true`)
- Stateless mode: full message history formatted per request
- Stateful mode: session continuity via `X-Session-ID` header
- Model override via `model` field

## Requirements
- Go 1.22+
- [Claude Code CLI](https://claude.ai/code) installed and authenticated (`claude auth login`)

## Build
```bash
go build -o cc_bridge .
```

## Run
```bash
./cc_bridge          # default port 8080
PORT=9000 ./cc_bridge
```

## Usage
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-code","messages":[{"role":"user","content":"Hello"}]}'
```

## Stateful sessions
Pass `X-Session-ID: <any-uuid>` header to maintain conversation context across requests.

