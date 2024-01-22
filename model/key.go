package model

import "time"

type Key struct {
	ID          string    `gorm:"column:id;primarykey" json:"id"`
	DisplayName string    `gorm:"column:display_name" json:"display_name"`
	UserID      string    `gorm:"column:user_id;index" json:"user_id"`
	CreatedAt   time.Time `gorm:"column:created_at;index" json:"created_at"`
}
