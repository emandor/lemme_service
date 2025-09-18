package providers

import (
	"context"
)

type Answer struct {
	Answer     string         `json:"answer"`
	Reason     string         `json:"reason,omitempty"`
	Options    []string       `json:"options,omitempty"`
	QuizID     int64          `json:"quiz_id"`
	Confidence float64        `json:"confidence,omitempty"`
	Raw        string         `json:"raw,omitempty"`
	LatencyMs  int            `json:"latency_ms,omitempty"`
	TokenUsage map[string]any `json:"token_usage,omitempty"`
}

type SourceName string

const (
	SourceOpenAI SourceName = "OPENAI"
	SourceClaude SourceName = "CLAUDE"
	SourceGemini SourceName = "GEMINI"
)

type Client interface {
	Name() SourceName
	Ask(ctx context.Context, prompt string) (Answer, error)
}
