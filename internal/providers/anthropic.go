package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/emandor/lemme_service/internal/telemetry"
)

type Anthropic struct {
	Key, Model string
	DryRun     bool
}

func (c *Anthropic) Name() SourceName { return SourceClaude }

func (c *Anthropic) Ask(ctx context.Context, prompt string) (Answer, error) {
	// DRY_RUN mode: skip API call
	if c.DryRun {
		log := telemetry.L().With().Str("provider", string(c.Name())).Logger()
		log.Info().Msg("openai_dry_run_enabled")

		parsed := Answer{
			QuizID:     0,
			Answer:     "simulated answer",
			Reason:     "simulated reason",
			Options:    []string{"A", "B", "C", "D"},
			Confidence: 0.9,
			Raw:        "",
			LatencyMs:  1,
			TokenUsage: map[string]any{
				"completion_tokens": 5,
			},
		}
		return parsed, nil
	}
	body := map[string]any{
		"model":      c.Model,
		"max_tokens": 512,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(b))
	req.Header.Set("x-api-key", c.Key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	t0 := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Answer{}, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Answer{}, errors.New("anthropic http " + resp.Status)
	}
	var out struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	_ = json.Unmarshal(raw, &out)
	if len(out.Content) == 0 {
		return Answer{}, errors.New("anthropic empty content")
	}

	parsed, _ := TryParseAnswer(out.Content[0].Text)
	parsed.LatencyMs = int(time.Since(t0) / time.Millisecond)
	return parsed, nil
}
