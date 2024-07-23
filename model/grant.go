package model

import "time"

type Grant struct {
	ID         string    `gorm:"column:id;primaryKey" json:"id"`
	UserID     string    `gorm:"column:user_id;index" json:"user_id"`
	ServerUser string    `gorm:"column:server_user;index" json:"server_user"`
	ServerID   string    `gorm:"column:server_id;index" json:"server_id"`
	CreatedAt  time.Time `gorm:"column:created_at;index" json:"created_at"`

	User User
}
