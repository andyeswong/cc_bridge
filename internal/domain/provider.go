package domain

import "strings"

type ProviderName string

const (
	ProviderClaude ProviderName = "claude"
	ProviderOllama ProviderName = "ollama"
)

const (
	DefaultClaudeModel = "claude-code"
	OllamaModelPrefix  = "ollama/"
)

func ResolveProvider(model string) ProviderName {
	if strings.HasPrefix(model, OllamaModelPrefix) {
		return ProviderOllama
	}

	return ProviderClaude
}

func StripOllamaPrefix(model string) string {
	return strings.TrimPrefix(model, OllamaModelPrefix)
}
