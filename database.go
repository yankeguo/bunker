package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/yankeguo/bunker/model"
	"github.com/yankeguo/bunker/model/dao"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

func initializeAdminUsers(dataDir DataDir, _db *gorm.DB) (err error) {
	buf, _ := os.ReadFile(filepath.Join(dataDir.String(), "admin.yaml"))
	buf = bytes.TrimSpace(buf)
	if len(buf) == 0 {
		return
	}

	var m map[string]string
	if err = yaml.Unmarshal(buf, &m); err != nil {
		return
	}

	db := dao.Use(_db)

	for username, password := range m {
		var user *model.User
		if user, err = db.User.Where(db.User.ID.Eq(username)).First(); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				user = &model.User{
					ID:        username,
					CreatedAt: time.Now(),
					VisitedAt: time.Now(),
					IsAdmin:   true,
				}
				user.SetPassword(password)

				if err = db.User.Create(user); err != nil {
					return
				}
			} else {
				return
			}
		} else {
			user.SetPassword(password)

			if _, err = db.User.Where(
				db.User.ID.Eq(username),
			).UpdateSimple(
				db.User.PasswordDigest.Value(user.PasswordDigest),
				db.User.IsAdmin.Value(true),
			); err != nil {
				return
			}
		}
	}

	return
}

func createDatabase(dataDir DataDir) (db *gorm.DB, err error) {
	if db, err = gorm.Open(
		sqlite.Open(filepath.Join(dataDir.String(), "database.sqlite3")),
		&gorm.Config{},
	); err != nil {
		return
	}
	if err = db.AutoMigrate(model.All...); err != nil {
		return
	}
	return
}
