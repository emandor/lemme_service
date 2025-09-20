package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv, AppPort, BaseURL string
	DBDSN                    string
	RedisAddr                string
	RedisDB                  int
	SessionCookieName        string
	SessionCookieSecret      string
	JWTSecret                string

	GoogleClientID, GoogleClientSecret, GoogleRedirectURL string
	OAuthAllowedDomains                                   []string
	CORSOrigins                                           []string

	OpenAIKey, OpenAIModel       string
	AnthropicKey, AnthropicModel string
	GeminiKey, GeminiModel       string

	OCRLang         string
	OCREngine       string
	OCROpenAIModel  string
	OCROpenAIKey    string
	OCRImgMaxW      int
	OCRImgQuality   int
	OCRImgGrayscale bool
	OCRCacheTTL     time.Duration

	OpenAIRPS          int
	OpenAIBurst        int
	ProviderMaxRetries int

	MaxBodyLimit       int
	AllowedMaxFileSize int
	AllowedFileExt     []string
}

func Load() *Config {
	_ = godotenv.Load()

	c := &Config{
		AppEnv:              get("APP_ENV", "dev"),
		AppPort:             get("APP_PORT", "8080"),
		BaseURL:             get("APP_BASE_URL", "http://localhost:8080"),
		DBDSN:               must("DB_DSN"),
		RedisAddr:           get("REDIS_ADDR", "127.0.0.1:6379"),
		RedisDB:             atoi(get("REDIS_DB", "0")),
		SessionCookieName:   get("SESSION_COOKIE_NAME", "lemme_sid"),
		SessionCookieSecret: must("SESSION_COOKIE_SECRET"),
		JWTSecret:           must("JWT_SECRET"),
		CORSOrigins:         split(get("CORS_ORIGINS", "http://localhost:5173")),
		GoogleClientID:      must("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:  must("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:   must("GOOGLE_REDIRECT_URL"),
		OAuthAllowedDomains: split(get("OAUTH_ALLOWED_DOMAINS", "")),
		OpenAIKey:           get("OPENAI_API_KEY", ""),
		OpenAIModel:         get("OPENAI_MODEL", "gpt-4o-mini"),
		AnthropicKey:        get("ANTHROPIC_API_KEY", ""),
		AnthropicModel:      get("ANTHROPIC_MODEL", "claude-3-5-sonnet-latest"),
		GeminiKey:           get("GEMINI_API_KEY", ""),
		GeminiModel:         get("GEMINI_MODEL", "gemini-2.5-pro"),
		OCRLang:             get("OCR_LANG", "eng+ind"),
		OCREngine:           get("OCR_ENGINE", "openai"),
		OCROpenAIModel:      get("OCR_OPENAI_MODEL", "gpt-4o-mini"),
		OCROpenAIKey:        get("OCR_OPENAI_KEY", get("OCR_OPENAI_MODEL", "")),
		OCRImgMaxW:          atoi(get("OCR_IMG_MAX_W", "1024")),
		OCRImgQuality:       atoi(get("OCR_IMG_QUALITY", "60")),
		OCRImgGrayscale:     parseBool(get("OCR_IMG_GRAYSCALE", "true")),
		OCRCacheTTL:         mustDuration(get("OCR_CACHE_TTL", "168h")),
		OpenAIRPS:           atoi(get("OPENAI_RPS", "2")),
		OpenAIBurst:         atoi(get("OPENAI_BURST", "2")),
		ProviderMaxRetries:  atoi(get("PROVIDER_MAX_RETRIES", "3")),
		AllowedMaxFileSize:  GetEnvInt("ALLOWED_MAX_FILE_SIZE", 2),
		AllowedFileExt:      GetEnvList("ALLOWED_FILE_EXT", []string{".jpg", ".jpeg", ".png"}),
	}
	return c
}

func GetEnvInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return d
}

func GetEnvList(k string, d []string) []string {
	if v := os.Getenv(k); v != "" {
		return strings.Split(v, ",")
	}
	return d
}

func get(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func must(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("missing env %s", k)
	}
	return v
}
func atoi(s string) int                   { i, _ := strconv.Atoi(s); return i }
func parseBool(s string) bool             { b, _ := strconv.ParseBool(s); return b }
func mustDuration(s string) time.Duration { d, _ := time.ParseDuration(s); return d }
func split(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func GetEnv(k, d string) string {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	return v
}
