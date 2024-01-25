package model

import "time"

type Token struct {
	ID        string    `gorm:"column:id;primarykey" json:"id"`
	UserID    string    `gorm:"column:user_id;not null;index" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null;index" json:"created_at"`
	VisitedAt time.Time `gorm:"column:visited_at;not null;index" json:"visited_at"`
	UserAgent string    `gorm:"column:user_agent;not null" json:"user_agent"`

	User User `json:"-"`
}
