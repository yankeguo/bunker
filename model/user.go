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

	Keys   []Key   `json:"keys,omitempty"`
	Grants []Grant `json:"grants,omitempty"`
	Tokens []Token `json:"tokens,omitempty"`
}

// CreateUserPassword returns a bcrypt hash of p.
func CreateUserPassword(p string) (string, error) {
	var u User
	if err := u.SetPassword(p); err != nil {
		return "", err
	}
	return u.PasswordDigest, nil
}

// SetPassword stores a bcrypt hash of p on u.
func (u *User) SetPassword(p string) error {
	b, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordDigest = string(b)
	return nil
}

// CheckPasswordDigest reports whether password matches a bcrypt digest.
func CheckPasswordDigest(digest, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(digest), []byte(password)) == nil
}

// CheckPassword reports whether p matches the user's password.
func (u *User) CheckPassword(p string) bool {
	if u == nil {
		return false
	}
	return CheckPasswordDigest(u.PasswordDigest, p)
}
