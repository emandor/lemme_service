package quiz

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"time"

	"github.com/emandor/lemme_service/internal/img"
	"github.com/emandor/lemme_service/internal/telemetry"
	ws "github.com/emandor/lemme_service/internal/ws"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/emandor/lemme_service/internal/ocr"
	"github.com/emandor/lemme_service/internal/providers"

	"golang.org/x/sync/errgroup"
)

type Service struct {
	db      *sqlx.DB
	rdb     *redis.Client
	clients []providers.Client
	ocrLang string
	ocr     interface {
		Read(ctx context.Context, img []byte, mime string) (ocr.Result, error)
	}
	ocrMaxW     int
	ocrQuality  int
	ocrGray     bool
	ocrCacheTTL time.Duration
}

func (s *Service) ProcessAsync(quizID int64, _imagePathIgnored string) {
	go func() {
		log := telemetry.L().With().Int64("quiz_id", quizID).Logger()
		log.Info().Str("stage", "start").Msg("process_quiz")

		ctx := context.Background()
		qid := strconv.FormatInt(quizID, 10)

		// lock redis (10 minutes), auto-release
		lockKey := "lock:quiz:" + qid
		ok, _ := s.rdb.SetNX(ctx, lockKey, "1", 10*time.Minute).Result()
		if !ok {
			log.Warn().Msg("lock_exists_skip")
			return
		}
		defer s.rdb.Del(ctx, lockKey)

		// get image info from DB (need path + hash)
		var row struct {
			ImagePath string `db:"image_path"`
			Hash      string `db:"image_hash"`
		}
		// if not found, mark error and return
		if err := s.db.Get(&row, `SELECT image_path, image_hash FROM quizzes WHERE id=?`, quizID); err != nil {
			log.Error().Err(err).Msg("quiz_not_found")
			s.markError(quizID, err)
			return
		}

		// find in redis using hash as key
		cacheKey := "ocr:" + row.Hash
		if txt, err := s.rdb.Get(ctx, cacheKey).Result(); err == nil && strings.TrimSpace(txt) != "" {
			log.Info().Int("len", len(txt)).Msg("ocr_cache_hit")
			s.saveOCR(quizID, txt)
		} else {
			log.Info().Str("img", row.ImagePath).Msg("ocr_cache_miss_preprocess")

			// Preprocess for efficient budget usage
			prep, err := img.PrepareForOCR(row.ImagePath, s.ocrMaxW, s.ocrQuality, s.ocrGray)
			if err != nil {
				log.Error().Err(err).Msg("ocr_prep_fail")
				s.markError(quizID, err)
				return
			}

			// call OCR service with 45s timeout
			ocrCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			defer cancel()
			res, err := s.ocr.Read(ocrCtx, prep.Bytes, prep.MIME)
			if err != nil {
				log.Error().Err(err).Msg("ocr_fail")
				s.markError(quizID, err)
				return
			}

			txt := strings.TrimSpace(res.Text)
			log.Info().Int("len", len(txt)).Msg("ocr_done")
			s.saveOCR(quizID, txt)

			if len(txt) > 0 && s.ocrCacheTTL > 0 {
				if err := s.rdb.Set(ctx, cacheKey, txt, s.ocrCacheTTL).Err(); err != nil {
					log.Warn().Err(err).Msg("ocr_cache_set_err")
				}
			}
		}

		// build prompt from latest OCR text
		txt := s.latestOCR(quizID)
		prompt := providers.BuildPrompt(txt)
		log.Debug().Int("prompt_len", len(prompt)).Msg("prompt_built")
		// debug prompt message
		log.Debug().Str("prompt", prompt).Msg("prompt_full")

		// Fan Out to each provider (text models)
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(min(len(s.clients), 3)) // mis. 3 concurrent;

		for _, cl := range s.clients {
			cli := cl // capture range var
			g.Go(func() error {
				// recover so that if 1 provider panics, it doesn't crash the whole process
				defer func() {
					if r := recover(); r != nil {
						log.Error().Str("provider", string(cli.Name())).Interface("panic", r).Msg("provider_panic")
					}
				}()

				askCtx, cancel := context.WithTimeout(gctx, 60*time.Second)
				defer cancel()

				ans, err := cli.Ask(askCtx, prompt)

				if err != nil {
					log.Error().Err(err).Str("provider", string(cli.Name())).Msg("provider_ask_error")

					// save "ERROR" answer but don't fail the whole process
					s.saveAnswer(quizID, cli.Name(), providers.Answer{Answer: "ERROR", Reason: err.Error()}, err)

					ws.BroadcastQuizUpdate(quizID, cli.Name(), nil, err)
					return nil
				}

				log.Info().Str("provider", string(cli.Name())).Int("len", len(ans.Answer)).Int("latency_ms", ans.LatencyMs).Msg("provider_done")

				s.saveAnswer(quizID, cli.Name(), ans, nil)
				ws.BroadcastQuizUpdate(quizID, cli.Name(), &ans, nil)
				return nil
			})
		}

		// wait for all providers to finish
		_ = g.Wait()

		s.markCompleted(quizID)
		go func() {
			for i := 0; i < 30; i++ {
				if ws.HasSubscribers(quizID) {
					ws.BroadcastQuizCompleted(quizID)
					return
				}
				time.Sleep(1 * time.Second)
			}
			// fallback anyway
			ws.BroadcastQuizCompleted(quizID)
		}()
		log.Info().Str("stage", "completed").Msg("process_quiz")
	}()
}

func (s *Service) saveOCR(quizID int64, text string) {
	_, _ = s.db.Exec(`UPDATE quizzes SET ocr_text=?, status='processing', updated_at=NOW() WHERE id=?`, text, quizID)

	go func() {
		for i := 0; i < 30; i++ { // try 30 times, 1s interval
			if ws.HasSubscribers(quizID) {
				ws.BroadcastQuizOCRDone(quizID, text)
				return
			}
			time.Sleep(1 * time.Second)
		}
		// fallback broadcast anyway
		ws.BroadcastQuizOCRDone(quizID, text)
	}()
}

func (s *Service) saveAnswer(quizID int64, source providers.SourceName, ans providers.Answer, err error) {
	if err != nil {
		_, _ = s.db.Exec(`INSERT INTO answers(quiz_id,source,answer_text,reason_text) VALUES(?,?,?,?)
			ON DUPLICATE KEY UPDATE reason_text=?`,
			quizID, source, "ERROR", err.Error(), err.Error())
		return
	}
	_, _ = s.db.Exec(`INSERT INTO answers(quiz_id,source,answer_text,reason_text,latency_ms,created_at)
			VALUES(?,?,?,?,?,NOW())
			ON DUPLICATE KEY UPDATE answer_text=VALUES(answer_text),
				reason_text=VALUES(reason_text),
				latency_ms=VALUES(latency_ms)`,
		quizID, source, ans.Answer, ans.Reason, ans.LatencyMs)
}

func (s *Service) markError(quizID int64, _ error) {
	_, _ = s.db.Exec(`UPDATE quizzes SET status='error', updated_at=NOW() WHERE id=?`, quizID)
}

func (s *Service) markCompleted(quizID int64) {
	_, _ = s.db.Exec(`UPDATE quizzes SET status='completed', updated_at=NOW() WHERE id=?`, quizID)
}

// latestOCR get ocr_text from DB (safe for prompt builder)
func (s *Service) latestOCR(quizID int64) string {
	var txt sql.NullString
	if err := s.db.Get(&txt, `SELECT ocr_text FROM quizzes WHERE id=?`, quizID); err != nil {
		return ""
	}
	if txt.Valid {
		return txt.String
	}
	return ""
}
