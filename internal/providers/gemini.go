package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/emandor/lemme_service/internal/telemetry"
)

type Gemini struct {
	Key, Model string
	DryRun     bool
}

func (c *Gemini) Name() SourceName { return SourceGemini }

func (c *Gemini) Ask(ctx context.Context, prompt string) (Answer, error) {
	// DRY_RUN mode: skip API call
	if c.DryRun {
		log := telemetry.L().With().Str("provider", string(c.Name())).Logger()
		log.Info().Msg("gemini_dry_run_enabled")
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
		"contents": []any{
			map[string]any{
				"role": "user",
				"parts": []any{
					map[string]string{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature":      0.0,
			"maxOutputTokens":  256,
			"responseMimeType": "application/json",
		},
	}

	b, errBody := json.Marshal(body)
	if errBody != nil {
		return Answer{}, errBody
	}

	log := telemetry.L().With().Str("provider", string(c.Name())).Int("body_len", len(b)).Logger()
	log.Debug().Msg("gemini_request")

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", c.Model)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-goog-api-key", c.Key)

	t0 := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("gemini_request_failed")
		return Answer{}, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	log.Debug().Int("status_code", resp.StatusCode).Int("body_len", len(raw)).Msg("gemini_response")

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Error().Str("status", resp.Status).RawJSON("body", raw).Msg("gemini_http_error")
		return Answer{}, errors.New("gemini http " + resp.Status)
	}

	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		PromptFeedback *struct {
			BlockReason string `json:"blockReason"`
		} `json:"promptFeedback"`
	}

	_ = json.Unmarshal(raw, &out)

	log.Debug().RawJSON("parsed", raw).Msg("gemini_parsed_response")

	if out.PromptFeedback != nil && out.PromptFeedback.BlockReason != "" {
		return Answer{}, errors.New("gemini blocked: " + out.PromptFeedback.BlockReason)
	}

	var text string
	if len(out.Candidates) > 0 && len(out.Candidates[0].Content.Parts) > 0 {
		text = out.Candidates[0].Content.Parts[0].Text
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return Answer{}, errors.New("gemini empty candidates")
	}

	parsed, _ := TryParseAnswer(text)
	parsed.LatencyMs = int(time.Since(t0) / time.Millisecond)
	return parsed, nil
}
