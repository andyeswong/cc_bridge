package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"claude-bridge/internal/domain"
)

type StreamEvent struct {
	Type    string         `json:"type"`
	Subtype string         `json:"subtype,omitempty"`
	IsError bool           `json:"is_error,omitempty"`
	Result  string         `json:"result,omitempty"`
	Message *StreamMessage `json:"message,omitempty"`
	Usage   *UsageResult   `json:"usage,omitempty"`
}

type StreamMessage struct {
	Content []StreamBlock `json:"content"`
}

type StreamBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type flusher interface {
	Flush()
}

func (p *Provider) Stream(ctx context.Context, writer io.Writer, req domain.ChatRequest, clientSessionID string, responseID string) (*domain.Usage, error) {
	input, cleanup, err := p.prepareCLIInput(req, clientSessionID, true)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	usage, err := p.runStream(ctx, writer, input, responseID, req.Model)
	if err != nil {
		return nil, err
	}

	return convertUsage(usage), nil
}

func (p *Provider) runStream(
	ctx context.Context,
	writer io.Writer,
	input cliArgs,
	responseID string,
	displayModel string,
) (*UsageResult, error) {
	if displayModel == "" {
		displayModel = domain.DefaultClaudeModel
	}

	args := buildArgs(p.cfg, input)

	cmd := exec.CommandContext(ctx, binaryPath(p.cfg), args...)
	cmd.Env = p.processEnv()
	cmd.Stdin = strings.NewReader(input.Prompt)

	workdir := p.cfg.ClaudeWorkdir
	if input.Workdir != "" {
		workdir = input.Workdir
	}

	if workdir != "" {
		cmd.Dir = workdir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("claude stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("claude start: %w", err)
	}

	var capturedUsage *UsageResult

	sendSSEChunk(writer, responseID, displayModel, "assistant", "", nil)
	flush(writer)

	scanner := bufio.NewScanner(stdout)

	buffer := make([]byte, 0, 1024*1024)
	scanner.Buffer(buffer, 8*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event StreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		switch event.Type {
		case "assistant":
			if event.Message == nil {
				continue
			}

			for _, block := range event.Message.Content {
				if block.Type != "text" || block.Text == "" {
					continue
				}

				sendSSEChunk(writer, responseID, displayModel, "", block.Text, nil)
				flush(writer)
			}

		case "result":
			if event.Usage != nil {
				capturedUsage = event.Usage
			}

			stop := "stop"
			sendSSEChunk(writer, responseID, displayModel, "", "", &stop)
			flush(writer)
		}
	}

	if err := scanner.Err(); err != nil {
		_ = cmd.Wait()
		return capturedUsage, fmt.Errorf("claude stream scan: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return capturedUsage, fmt.Errorf("claude wait: %w", err)
	}

	_, _ = fmt.Fprint(writer, "data: [DONE]\n\n")
	flush(writer)

	return capturedUsage, nil
}

func sendSSEChunk(
	writer io.Writer,
	id string,
	model string,
	role string,
	content string,
	finishReason *string,
) {
	chunk := domain.NewStreamChunk(
		id,
		model,
		role,
		content,
		finishReason,
	)

	payload, err := json.Marshal(chunk)
	if err != nil {
		return
	}

	_, _ = fmt.Fprintf(writer, "data: %s\n\n", payload)
}

func flush(writer io.Writer) {
	if f, ok := writer.(flusher); ok {
		f.Flush()
	}
}
