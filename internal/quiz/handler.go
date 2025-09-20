package quiz

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/emandor/lemme_service/internal/config"
	"github.com/emandor/lemme_service/internal/img"
	"github.com/emandor/lemme_service/internal/middleware"
	"github.com/emandor/lemme_service/internal/model"
	"github.com/emandor/lemme_service/internal/ocr"
	"github.com/emandor/lemme_service/internal/providers"
	"github.com/emandor/lemme_service/internal/quota"
	"github.com/emandor/lemme_service/internal/telemetry"
	"github.com/emandor/lemme_service/internal/ws"
)

type Handler struct {
	cfg *config.Config
	db  *sqlx.DB
	rdb *redis.Client
	svc *Service
}

func buildProviders(cfg *config.Config) []providers.Client {
	var list []providers.Client
	// set to DRY_RUN mode for testing without API calls
	dryRun := false
	if cfg.OpenAIKey != "" {
		list = append(list, &providers.OpenAI{Key: cfg.OpenAIKey, Model: cfg.OpenAIModel, DryRun: dryRun})
	}
	if cfg.AnthropicKey != "" {
		list = append(list, &providers.Anthropic{Key: cfg.AnthropicKey, Model: cfg.AnthropicModel, DryRun: dryRun})
	}
	if cfg.GeminiKey != "" {
		list = append(list, &providers.Gemini{Key: cfg.GeminiKey, Model: cfg.GeminiModel, DryRun: dryRun})
	}
	return list
}

func NewHandler(cfg *config.Config, db *sqlx.DB, rdb *redis.Client) *Handler {
	clients := buildProviders(cfg) // init OpenAI/Anthropic/DeepSeek
	svc := &Service{db: db, rdb: rdb, clients: clients, ocrLang: cfg.OCRLang}

	vision := ocr.NewOpenAIVision(
		cfg.OpenAIKey,
		cfg.OCROpenAIModel,
		cfg.OpenAIRPS,
		cfg.OpenAIBurst,
		cfg.ProviderMaxRetries,
	)

	svc.ocr = vision
	svc.ocrMaxW = cfg.OCRImgMaxW
	svc.ocrQuality = cfg.OCRImgQuality
	svc.ocrGray = cfg.OCRImgGrayscale
	svc.ocrCacheTTL = cfg.OCRCacheTTL
	return &Handler{cfg: cfg, db: db, rdb: rdb, svc: svc}
}

func (h *Handler) CreateQuiz(c *fiber.Ctx) error {
	userID := mustUserID(c)

	rid := c.Locals(middleware.ReqIDKey).(string)
	userid := mustUserID(c)
	log := telemetry.L().With().Str("req_id", rid).Int64("user_id", userid).Logger()

	var u model.User
	if err := h.db.Get(&u, `SELECT id, quiz_quota, quiz_used FROM users WHERE id=?`, userID); err != nil {
		return c.Status(500).SendString("db error")
	}

	uq := quota.UserQuota{QuizQuota: u.QuizQuota, QuizUsed: u.QuizUsed}
	if !uq.CanCreateQuiz() {
		return c.Status(403).SendString("quota exceeded")
	}

	if err := h.db.Get(&u, `SELECT id, quiz_quota, quiz_used FROM users WHERE id=?`, userID); err != nil {
		return c.Status(500).SendString("db error")
	}

	fh, err := c.FormFile("image")
	if err != nil {
		return c.Status(400).SendString("image required")
	}

	uid := uuid.New().String()
	tmp := filepath.Join(os.TempDir(), uid)
	if err := c.SaveFile(fh, tmp); err != nil {
		return c.Status(500).SendString("save fail")
	}

	save, err := img.SaveResizedJPEG(tmp, "./storage/quizzes", 700)
	if err != nil {
		return c.Status(500).SendString("resize fail")
	}

	var id int64
	res, err := h.db.Exec(`
  INSERT INTO quizzes
    (user_id, title, image_path, image_hash, image_width, image_height, status, created_at, updated_at)
  VALUES
    (?, NULL, ?, ?, ?, ?, 'processing', NOW(), NOW())
`, userID, save.Path, save.Hash, save.Width, save.Height)
	if err != nil {
		return c.Status(500).SendString("db fail")
	}

	qid, _ := res.LastInsertId()
	log.Info().Int64("quiz_id", id).Msg("quiz_created")
	// need to broadcast new quiz to user via websocket
	ws.BroadcastNewQuiz(userID, qid, save.Path)

	// Async process
	h.svc.ProcessAsync(qid, save.Path)
	_, _ = h.db.Exec(`UPDATE users SET quiz_used=quiz_used+1 WHERE id=?`, userID)
	return c.JSON(fiber.Map{"id": qid, "status": "processing", "image_path": save.Path})
}

type QuizRow struct {
	ID        int64       `db:"id" json:"id"`
	Status    string      `db:"status" json:"status"`
	OcrText   string      `db:"ocr_text" json:"ocr_text"`
	ImagePath string      `db:"image_path" json:"image_path"`
	CreatedAt string      `db:"created_at" json:"created_at"`
	Answers   []AnswerRow `json:"answers"`
}

type AnswerRow struct {
	Source    string `db:"source" json:"source"`
	Answer    string `db:"answer_text" json:"answer_text"`
	Reason    string `db:"reason_text" json:"reason_text"`
	CreatedAt string `db:"created_at" json:"created_at"`
}

func (h *Handler) ListMyQuizzes(c *fiber.Ctx) error {
	userID := mustUserID(c)

	var quizzes []QuizRow
	if err := h.db.Select(&quizzes, `
        SELECT id,status,ocr_text,image_path,created_at
        FROM quizzes
        WHERE user_id=? ORDER BY id DESC`, userID); err != nil {
		return c.Status(500).SendString("db fail")
	}

	for i := range quizzes {
		var answers []AnswerRow
		_ = h.db.Select(&answers, `
	           SELECT source,answer_text,reason_text,created_at
	           FROM answers WHERE quiz_id=?
	           ORDER BY id ASC`, quizzes[i].ID)
		quizzes[i].Answers = answers
	}

	return c.JSON(quizzes)
}

func (h *Handler) GetQuiz(c *fiber.Ctx) error {
	userID := mustUserID(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	var q struct {
		ID                         int64
		UserID                     int64
		Status, OCRText, ImagePath string
	}
	if err := h.db.Get(&q, `SELECT id,user_id,status,ocr_text,image_path FROM quizzes WHERE id=?`, id); err != nil {
		return c.Status(404).SendString("not found")
	}
	if q.UserID != userID {
		return c.Status(403).SendString("forbidden")
	}
	return c.JSON(q)
}

func (h *Handler) ListAnswers(c *fiber.Ctx) error {
	userID := mustUserID(c)
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	var owner int64
	if err := h.db.Get(&owner, `SELECT user_id FROM quizzes WHERE id=?`, id); err != nil {
		return c.Status(404).SendString("not found")
	}
	if owner != userID {
		return c.Status(403).SendString("forbidden")
	}
	var rows []struct {
		Source    string `db:"source"`
		Answer    string `db:"answer_text"`
		Reason    string `db:"reason_text"`
		CreatedAt string `db:"created_at"`
	}
	_ = h.db.Select(&rows, `SELECT source,answer_text,reason_text,created_at FROM answers WHERE quiz_id=? ORDER BY id ASC`, id)
	return c.JSON(rows)
}

func mustUserID(c *fiber.Ctx) int64 {
	uid, ok := c.Locals("userID").(int64)
	if !ok {
		return 0
	}
	return uid
}
