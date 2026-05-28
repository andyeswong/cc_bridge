package claude

import (
	"testing"

	"claude-bridge/internal/domain"
)

func TestFormatMessages(t *testing.T) {
	t.Parallel()

	got := formatMessages([]domain.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "yo"},
	})

	want := "<system>\nsys\n</system>\n\nHuman: hi\n\nAssistant: yo"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestLastUserMessage(t *testing.T) {
	t.Parallel()

	if got := lastUserMessage([]domain.Message{{Role: "assistant", Content: "x"}}); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}

	got := lastUserMessage([]domain.Message{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
		{Role: "user", Content: "c"},
	})
	if got != "c" {
		t.Fatalf("got %q want %q", got, "c")
	}
}
