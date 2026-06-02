package external

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"claude-bridge/internal/domain"

	"github.com/google/uuid"
)

type flusher interface {
	Flush()
}

func (p *Provider) Stream(
	ctx context.Context,
	writer io.Writer,
	req domain.ChatRequest,
	_ string,
	responseID string,
) (*domain.Usage, error) {
	if responseID == "" {
		responseID = "chatcmpl-" + uuid.NewString()
	}

	body := requestBody{
		Model:      req.Model,
		Messages:   req.Messages,
		Stream:     true,
		Tools:      req.Tools,
		ToolChoice: req.ToolChoice,
	}

	rawBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("external stream marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint(), bytes.NewReader(rawBody))
	if err != nil {
		return nil, fmt.Errorf("external stream create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.ep.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.ep.APIKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("external stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		rawErrBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("external stream status %d: %s", resp.StatusCode, strings.TrimSpace(string(rawErrBody)))
	}

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 8*1024*1024)

	var capturedUsage *domain.Usage

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "data: ") && line != "data: [DONE]" {
			data := strings.TrimPrefix(line, "data: ")
			var chunk domain.ChatResponse
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				if chunk.Usage != nil {
					capturedUsage = chunk.Usage
				}
				if chunk.ID == "" {
					chunk.ID = responseID
				}
				if chunk.Model == "" {
					chunk.Model = req.Model
				}
				if rewritten, err := json.Marshal(chunk); err == nil {
					line = "data: " + string(rewritten)
				}
			}
		}

		if line == "" {
			fmt.Fprint(writer, "\n")
		} else {
			fmt.Fprintf(writer, "%s\n", line)
		}
		flush(writer)
	}

	if err := scanner.Err(); err != nil {
		return capturedUsage, fmt.Errorf("external stream scan: %w", err)
	}

	return capturedUsage, nil
}

func flush(w io.Writer) {
	if f, ok := w.(flusher); ok {
		f.Flush()
	}
}
