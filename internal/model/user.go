package model

import "time"

type User struct {
	ID          int64     `db:"id"`
	Provider    string    `db:"provider"`
	ProviderID  string    `db:"provider_id"`
	Email       string    `db:"email"`
	Name        string    `db:"name"`
	Picture     string    `db:"picture"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	LastLoginAt time.Time `db:"last_login_at"`
	QuizQuota   int       `db:"quiz_quota"`
	QuizUsed    int       `db:"quiz_used"`
}
