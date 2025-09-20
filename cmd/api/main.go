package main

import (
	"flag"
	"log"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"

	"github.com/emandor/lemme_service/internal/auth"
	"github.com/emandor/lemme_service/internal/cache"
	"github.com/emandor/lemme_service/internal/config"
	"github.com/emandor/lemme_service/internal/db"
	"github.com/emandor/lemme_service/internal/middleware"
	"github.com/emandor/lemme_service/internal/quiz"
	"github.com/emandor/lemme_service/internal/telemetry"
	"github.com/emandor/lemme_service/internal/ws"
)

func main() {
	doMigrate := flag.Bool("migrate", false, "run migrations and exit")
	flag.Parse()

	cfg := config.Load()
	sqlxDB := db.MustConnect(cfg.DBDSN)
	rdb := cache.MustConnect(cfg.RedisAddr, cfg.RedisDB)

	tlog := telemetry.Init(telemetry.FromEnv(config.GetEnv))
	tlog.Info().Str("port", cfg.AppPort).Msg("booting lemme_service")

	if *doMigrate {
		db.MustMigrate(sqlxDB)
		log.Println("migrations done")
		return
	}

	app := fiber.New()

	// app.Use(middleware.RateLimiter())
	app.Use(middleware.RequestID())
	app.Use(middleware.Recover())
	app.Use(middleware.CORS(cfg))
	app.Use(middleware.RequestLog())
	// app.Use(middleware.WSUpgradeMiddleware())
	// app.Use(middleware.SecureHeaders())

	authReg := auth.NewRegistry(cfg, sqlxDB, rdb)

	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	app.Static("/storage", "./storage")
	app.Get("/api/v1/auth/google/login", authReg.GoogleLogin)
	app.Get("/api/v1/auth/google/callback", authReg.GoogleCallback)

	qh := quiz.NewHandler(cfg, sqlxDB, rdb)
	protected := app.Group("/api/v1", middleware.AuthSession(authReg))

	protected.Post("/auth/logout", authReg.Logout)
	protected.Get("/me", authReg.Me)

	protected.Post("/quizzes", middleware.FileUploadValidator(cfg), qh.CreateQuiz)
	protected.Get("/quizzes", qh.ListMyQuizzes)
	protected.Get("/quizzes/:id", qh.GetQuiz)
	protected.Get("/quizzes/:id/answers", qh.ListAnswers)

	app.Get("/ws", websocket.New(ws.HandleWS))

	log.Fatal(app.Listen(":" + cfg.AppPort))
}
