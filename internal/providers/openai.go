package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/emandor/lemme_service/internal/telemetry"
)

type OpenAI struct {
	Key, Model string
	DryRun     bool
}

func (c *OpenAI) Name() SourceName { return SourceOpenAI }

func (c *OpenAI) Ask(ctx context.Context, prompt string) (Answer, error) {
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
				"prompt_tokens":     len(strings.Fields(prompt)),
				"completion_tokens": 5,
			},
		}
		return parsed, nil
	}
	body := map[string]any{
		"model":             c.Model,
		"input":             prompt,
		"temperature":       0.0,
		"max_output_tokens": 256,
	}

	b, _ := json.Marshal(body)
	log := telemetry.L().With().Str("provider", string(c.Name())).Int("body_len", len(b)).Logger()
	// redacted log API key
	b = bytes.ReplaceAll(b, []byte(c.Key), []byte("REDACTED"))

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/responses", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Content-Type", "application/json")

	t0 := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("openai_request_failed")
		return Answer{}, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	log.Debug().Int("body_len", len(raw)).Msg("openai_response")

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Error().
			Str("status", resp.Status).
			RawJSON("body", raw).
			Msg("openai_http_error")
		return Answer{}, errors.New("openai http " + resp.Status)
	}

	// parse: responses API: fallback to chat completions
	text := extractOpenAIText(raw)
	if strings.TrimSpace(text) == "" {
		// no fatal panic; send an error that can be handled by the caller
		return Answer{}, errors.New("openai: empty text")
	}

	parsed, _ := TryParseAnswer(text)
	parsed.LatencyMs = int(time.Since(t0) / time.Millisecond)

	// usage (if any) → put into TokenUsage
	var u struct {
		Usage map[string]any `json:"usage"`
	}
	if json.Unmarshal(raw, &u) == nil && u.Usage != nil {
		parsed.TokenUsage = u.Usage
	}

	return parsed, nil
}

// get text from Responses API or fallback Chat Completions.
func extractOpenAIText(raw []byte) string {
	// resonses API: https://platform.openai.com/docs/api-reference/responses
	var r1 struct {
		OutputText string `json:"output_text"`
	}
	if json.Unmarshal(raw, &r1) == nil && strings.TrimSpace(r1.OutputText) != "" {
		return r1.OutputText
	}

	// response API: output[].content[].text
	var r2 struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if json.Unmarshal(raw, &r2) == nil && len(r2.Output) > 0 {
		for _, c := range r2.Output[0].Content {
			if strings.TrimSpace(c.Text) != "" {
				return c.Text
			}
		}
	}

	// fallback chat completions format: choices[0].message.content
	var r3 struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(raw, &r3) == nil && len(r3.Choices) > 0 {
		return r3.Choices[0].Message.Content
	}

	return ""
}
