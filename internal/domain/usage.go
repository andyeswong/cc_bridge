package domain

type UsageLogInput struct {
	Model            string
	Provider         ProviderName
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	DurationMs       int
	IsError          bool
}

func NewUsageFromClaude(inputTokens int, outputTokens int) *Usage {
	return &Usage{
		PromptTokens:     inputTokens,
		CompletionTokens: outputTokens,
		TotalTokens:      inputTokens + outputTokens,
	}
}

func EmptyUsage() *Usage {
	return &Usage{
		PromptTokens:     0,
		CompletionTokens: 0,
		TotalTokens:      0,
	}
}
