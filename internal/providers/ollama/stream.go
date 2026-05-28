package ollama

import (
	"bufio"
	"bytes"
	"claude-bridge/internal/domain"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type flusher interface {
	Flush()
}

func (p *Provider) Stream(
	ctx context.Context,
	writer io.Writer,
	req domain.ChatRequest,
	clientSessionID string,
	responseID string,
) (*domain.Usage, error) {
	displayModel := req.Model
	ollamaModel := resolveOllamaModel(req.Model)

	if responseID == "" {
		responseID = "chatcmpl-" + uuid.NewString()
	}

	messages := p.buildMessages(req.Messages, clientSessionID)

	assistantText, usage, err := p.runStream(
		ctx,
		writer,
		ollamaModel,
		messages,
		responseID,
		displayModel,
	)
	if err != nil {
		return usage, err
	}

	if clientSessionID != "" && assistantText != "" {
		updated := append(messages, domain.Message{
			Role:    "assistant",
			Content: assistantText,
		})

		p.sessions.Set(clientSessionID, trimHistory(updated, p.maxHistory))
	}

	return usage, nil
}

func (p *Provider) runStream(
	ctx context.Context,
	writer io.Writer,
	model string,
	messages []domain.Message,
	responseID string,
	displayModel string,
) (string, *domain.Usage, error) {
	payload := requestBody{
		Model:    model,
		Messages: messages,
		Stream:   true,
		Options: Options{
			NumCtx: defaultNumCtx,
		},
	}

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return "", nil, fmt.Errorf("ollama marshal stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.chatEndpoint(),
		bytes.NewReader(rawPayload),
	)
	if err != nil {
		return "", nil, fmt.Errorf("ollama create stream request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return "", nil, fmt.Errorf("ollama stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		rawBody, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("ollama stream status %d: %s", resp.StatusCode, strings.TrimSpace(string(rawBody)))
	}

	scanner := bufio.NewScanner(resp.Body)

	buffer := make([]byte, 0, 1024*1024)
	scanner.Buffer(buffer, 8*1024*1024)

	var assistantText strings.Builder
	var capturedUsage *domain.Usage

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		if data == "[DONE]" {
			_, _ = fmt.Fprint(writer, "data: [DONE]\n\n")
			flush(writer)
			break
		}

		var chunk domain.ChatResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		chunk.ID = responseID

		if chunk.Model == "" {
			chunk.Model = displayModel
		}

		if chunk.Usage != nil {
			capturedUsage = chunk.Usage
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
			assistantText.WriteString(chunk.Choices[0].Delta.Content)
		}

		rawChunk, err := json.Marshal(chunk)
		if err != nil {
			continue
		}

		_, _ = fmt.Fprintf(writer, "data: %s\n\n", rawChunk)
		flush(writer)
	}

	if err := scanner.Err(); err != nil {
		return assistantText.String(), capturedUsage, fmt.Errorf("ollama stream scan: %w", err)
	}

	return assistantText.String(), capturedUsage, nil
}

func flush(writer io.Writer) {
	if f, ok := writer.(flusher); ok {
		f.Flush()
	}
}
