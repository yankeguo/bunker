package model

import "time"

// HostKey is a target server host key pinned on first use.
// One row is kept per server and key type.
type HostKey struct {
	ID          string    `gorm:"column:id;primaryKey" json:"id"`
	ServerID    string    `gorm:"column:server_id;not null;index" json:"server_id"`
	KeyType     string    `gorm:"column:key_type;not null" json:"key_type"`
	Fingerprint string    `gorm:"column:fingerprint;not null" json:"fingerprint"`
	PublicKey   string    `gorm:"column:public_key;not null" json:"-"`
	CreatedAt   time.Time `gorm:"column:created_at;index" json:"created_at"`
}
