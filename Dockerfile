ARG GO_VERSION=1.25

FROM golang:${GO_VERSION}-bookworm AS builder

WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends \
  build-essential \
  ca-certificates \
  && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=1
ENV GOOS=linux

RUN go build -trimpath -ldflags="-s -w" -o /out/claude-bridge ./cmd/claude-bridge

FROM debian:bookworm-slim AS runtime

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
  ca-certificates \
  sqlite3 \
  && rm -rf /var/lib/apt/lists/*

RUN useradd -m -u 10001 appuser \
  && mkdir -p /data \
  && chown -R appuser:appuser /app /data

COPY --from=builder /out/claude-bridge /app/claude-bridge

USER appuser

ENV HOST=0.0.0.0
ENV PORT=8080
ENV USAGE_DB_PATH=/data/usage.db
ENV OLLAMA_URL=http://host.docker.internal:11434
ENV CLAUDE_DEFAULT_MODEL=claude-code
ENV CLAUDE_SKIP_PERMS=false

EXPOSE 8080

CMD ["/app/claude-bridge"]
