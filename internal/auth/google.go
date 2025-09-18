package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/emandor/lemme_service/internal/config"
	"github.com/emandor/lemme_service/internal/middleware"
	"github.com/emandor/lemme_service/internal/telemetry"
	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Registry struct {
	cfg   *config.Config
	db    *sqlx.DB
	rdb   *redis.Client
	oauth *oauth2.Config
}

func (r *Registry) Rdb() *redis.Client {
	return r.rdb
}

func (r *Registry) CookieName() string {
	return r.cfg.SessionCookieName
}

func NewRegistry(cfg *config.Config, db *sqlx.DB, rdb *redis.Client) *Registry {
	return &Registry{
		cfg: cfg, db: db, rdb: rdb,
		oauth: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

func (r *Registry) Logout(c *fiber.Ctx) error {
	sid := c.Cookies(r.cfg.SessionCookieName)
	if sid != "" {
		r.rdb.Del(c.Context(), "sess:"+sid)
		c.ClearCookie(r.cfg.SessionCookieName)
	}
	return c.SendString("ok")
}

func (r *Registry) Me(c *fiber.Ctx) error {
	uid := c.Locals("userID").(int64)
	var user struct {
		ID        int64     `db:"id" json:"id"`
		Email     string    `db:"email" json:"email"`
		Name      string    `db:"name" json:"name"`
		Picture   string    `db:"picture" json:"picture"`
		CreatedAt time.Time `db:"created_at" json:"created_at"`
	}
	err := r.db.Get(&user, `SELECT id, email, name, picture, created_at FROM users WHERE id=? LIMIT 1`, uid)
	if err != nil {
		return c.Status(500).SendString("db error")
	}
	return c.JSON(user)

}

func (r *Registry) GoogleLogin(c *fiber.Ctx) error {
	log := telemetry.L()
	log.Info().
		Str("req_id", c.Locals(middleware.ReqIDKey).(string)).
		Msg("google_login_redirect")
	state := randomHex(16)
	c.Cookie(&fiber.Cookie{Name: "oauth_state", Value: state, HTTPOnly: true, Secure: false, SameSite: "Lax"})
	url := r.oauth.AuthCodeURL(state, oauth2.AccessTypeOnline)
	return c.Redirect(url, http.StatusFound)
}

func (r *Registry) GoogleCallback(c *fiber.Ctx) error {
	rid := c.Locals(middleware.ReqIDKey).(string)
	state := c.Cookies("oauth_state")
	log := telemetry.L().With().Str("req_id", rid).Logger()
	if state == "" || state != c.Query("state") {
		log.Warn().Str("req_id", rid).Msg("oauth_state_mismatch")
		return c.Status(400).SendString("bad state")
	}
	tok, err := r.oauth.Exchange(context.Background(), c.Query("code"))
	if err != nil {
		log.Error().Str("req_id", rid).Err(err).Msg("oauth_exchange_failed")
		return c.Status(400).SendString("exchange failed")
	}

	// fetch userinfo (id, email, name, picture)
	// call https://www.googleapis.com/oauth2/v3/userinfo with token
	ui, err := fetchGoogleUserinfo(tok.AccessToken)

	if len(r.cfg.OAuthAllowedDomains) > 0 {
		ok := false
		for _, d := range r.cfg.OAuthAllowedDomains {
			if strings.HasSuffix(strings.ToLower(ui.Email), "@"+strings.ToLower(d)) {
				ok = true
				break
			}
		}
		if !ok {
			return c.Status(403).SendString("domain not allowed")
		}
	}

	log.Info().
		Str("req_id", rid).
		Str("email", ui.Email).
		Str("sub", ui.Sub).
		Msg("login_userinfo")

	userID := upsertUser(r.db, ui) // set last_login_at = NOW()
	log.Info().Str("req_id", rid).Int64("user_id", userID).Msg("user_upserted")
	// upsert users + log session
	sessID := randomHex(16)
	saveSessionDB(r.db, sessID, userID, c.IP(), string(c.Request().Header.UserAgent()))

	// save session to Redis (TTL 7 days)
	ctx := context.Background()
	r.rdb.Set(ctx, "sess:"+sessID, userID, 7*24*time.Hour)

	// set cookie session
	c.Cookie(&fiber.Cookie{
		Name: r.cfg.SessionCookieName, Value: sessID, HTTPOnly: true, SameSite: "Lax", Secure: false, MaxAge: int((7 * 24 * time.Hour).Seconds()),
	})
	redir := c.Query("redirect")
	if redir == "" {
		// fallback
		redir = os.Getenv("CLIENT_URL") + "/login"
	}
	return c.Redirect(redir, http.StatusFound)
}

func randomHex(n int) string { b := make([]byte, n); rand.Read(b); return hex.EncodeToString(b) }

// fetchGoogleUserinfo call
type googleUserInfo struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func fetchGoogleUserinfo(accessToken string) (*googleUserInfo, error) {
	req, _ := http.NewRequest("GET",
		"https://www.googleapis.com/oauth2/v3/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var ui googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&ui); err != nil {
		return nil, err
	}
	return &ui, nil
}

func upsertUser(db *sqlx.DB, ui *googleUserInfo) int64 {
	log := telemetry.L().With().Str("email", ui.Email).Str("sub", ui.Sub).Logger()
	res, err := db.Exec(`
		INSERT INTO users (provider, provider_id, email, name, picture, last_login_at, created_at, updated_at)
		VALUES ('google', ?, ?, ?, ?, NOW(), NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			email = VALUES(email),
			name = VALUES(name),
			picture = VALUES(picture),
			last_login_at = NOW(),
			updated_at = NOW(),
			-- trik penting: set LAST_INSERT_ID ke id eksisting agar bisa diambil via LastInsertId()
			id = LAST_INSERT_ID(id)
	`, ui.Sub, ui.Email, ui.Name, ui.Picture)
	if err != nil {
		log.Fatal().Err(err).Msg("upsertUser failed")
	}

	id, err := res.LastInsertId()
	if err != nil || id == 0 {
		// fallback super-safety (rarely needed, but just in case)
		var fetched int64
		if e := db.Get(&fetched, `SELECT id FROM users WHERE provider='google' AND provider_id=? LIMIT 1`, ui.Sub); e != nil {
			log.Fatal().Err(e).Msg("fetch user id failed")
		}
		return fetched
	}
	return id
}

func saveSessionDB(db *sqlx.DB, sid string, userID int64, ip, ua string) {
	_, err := db.Exec(`INSERT INTO user_sessions(id,user_id,ip,user_agent) VALUES(?,?,?,?)`,
		sid, userID, ip, ua)
	if err != nil {
		log := telemetry.L().With().Int64("user_id", userID).Str("session_id", sid).Logger()
		log.Error().Err(err).Msg("saveSessionDB failed")
	}
}
