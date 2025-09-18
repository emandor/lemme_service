package ocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"github.com/emandor/lemme_service/internal/telemetry"
)

type OpenAIVision struct {
	Key, Model string
	Client     *http.Client
	Limiter    *rate.Limiter
	MaxRetries int
}

func NewOpenAIVision(key, model string, rps, burst, maxRetries int) *OpenAIVision {
	if rps <= 0 {
		rps = 2
	}
	if burst <= 0 {
		burst = 2
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &OpenAIVision{
		Key:        key,
		Model:      model,
		Client:     &http.Client{Timeout: 60 * time.Second},
		Limiter:    rate.NewLimiter(rate.Limit(rps), burst),
		MaxRetries: maxRetries,
	}
}

type Result struct {
	Text       string
	Confidence float64
	Raw        string
}

func (o *OpenAIVision) Read(ctx context.Context, imgB []byte, mime string) (Result, error) {
	if err := o.Limiter.Wait(ctx); err != nil {
		return Result{}, err
	}

	// data URL; use detail:"low" for low cost
	dataURL := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(imgB)
	payload := map[string]any{
		"model": o.Model,
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]string{"type": "text", "text": "Extract plain text (OCR). Return ONLY the raw text (no explanation)."},
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": dataURL, "detail": "low"}},
				},
			},
		},
		"temperature": 0.0,
		"max_tokens":  512, // adjust as needed; we need to limit cost
	}

	b, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+o.Key)
	req.Header.Set("Content-Type", "application/json")

	var lastErr error
	start := time.Now()
	for attempt := 0; attempt <= o.MaxRetries; attempt++ {
		if attempt > 0 {
			d := time.Duration(200*(1<<uint(attempt-1))) * time.Millisecond
			time.Sleep(d)
		}

		resp, err := o.Client.Do(req.Clone(ctx))
		if err != nil {
			lastErr = err
			continue
		}

		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		log := telemetry.L().With().Str("provider", "openai-vision").Logger()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var out struct {
				Choices []struct{ Message struct{ Content string } }
			}
			if err := json.Unmarshal(raw, &out); err != nil {
				return Result{Raw: string(raw)}, err
			}
			if len(out.Choices) == 0 {
				return Result{Raw: string(raw)}, errors.New("openai vision: empty choices")
			}
			txt := out.Choices[0].Message.Content
			log.Debug().Int("latency_ms", int(time.Since(start)/time.Millisecond)).Int("chars", len(txt)).Msg("ocr_ok")
			return Result{Text: txt, Raw: string(raw)}, nil
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			log.Warn().Int("status", resp.StatusCode).Msg("ocr_429_retry")
			lastErr = errors.New("openai vision 429")
			continue
		}

		lastErr = errors.New("openai vision http " + resp.Status)
		break
	}
	return Result{}, lastErr
}
