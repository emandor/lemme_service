package quota

import "errors"

var ErrQuotaExceeded = errors.New("quiz quota exceeded")

type QuotaChecker interface {
	CanCreateQuiz() bool
	IncrementUsed() error
	GetUsed() int
	GetQuota() int
}

type UserQuota struct {
	QuizQuota int
	QuizUsed  int
}

func (u *UserQuota) CanCreateQuiz() bool {
	return u.QuizUsed < u.QuizQuota
}

func (u *UserQuota) IncrementUsed() error {
	if !u.CanCreateQuiz() {
		return ErrQuotaExceeded
	}
	u.QuizUsed++
	return nil
}

func (u *UserQuota) GetUsed() int  { return u.QuizUsed }
func (u *UserQuota) GetQuota() int { return u.QuizQuota }
