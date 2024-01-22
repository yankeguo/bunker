package model

import (
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// UserIDPattern general name pattern
var UserIDPattern = regexp.MustCompile(`^[a-z][a-z0-9\._\-]{3,}$`)

type User struct {
	ID             string    `gorm:"column:id;primarykey" json:"id"`
	PasswordDigest string    `gorm:"column:password_digest;not null" json:"-"`
	CreatedAt      time.Time `gorm:"column:created_at;not null;index" json:"created_at"`
	VisitedAt      time.Time `gorm:"column:visited_at;not null;index" json:"visited_at"`
	IsAdmin        bool      `gorm:"column:is_admin;not null;default:0;index" json:"is_admin"`
	IsBlocked      bool      `gorm:"column:is_blocked;not null;default:0;index" json:"is_blocked"`
}

// SetPassword update password for user
// bcrypt produces clear text encrypted password, no further encoding needed
func (u *User) SetPassword(p string) (err error) {
	var b []byte
	if b, err = bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost); err != nil {
		return
	}
	u.PasswordDigest = string(b)
	return
}

// CheckPassword check password
func (u *User) CheckPassword(p string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordDigest), []byte(p)) == nil
}
