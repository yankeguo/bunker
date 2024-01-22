package main

import (
	"path/filepath"

	"github.com/glebarez/sqlite"
	"github.com/yankeguo/bunker/model"
	"gorm.io/gorm"
)

func createDatabase(dataDir DataDir) (db *gorm.DB, err error) {
	if db, err = gorm.Open(
		sqlite.Open(filepath.Join(dataDir.String(), "database.sqlite3")),
		&gorm.Config{},
	); err != nil {
		return
	}
	if err = db.Debug().AutoMigrate(model.All...); err != nil {
		return
	}
	return
}
