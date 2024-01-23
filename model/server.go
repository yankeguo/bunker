package model

import "time"

type Server struct {
	// ID will be user@server_name
	ID        string    `gorm:"column:id;primaryKey" json:"id" `
	Address   string    `gorm:"column:address" json:"address"`
	CreatedAt time.Time `gorm:"column:created_at;index" json:"created_at"`
}
