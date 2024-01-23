package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/yankeguo/bunker/model"
	"github.com/yankeguo/bunker/model/dao"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

func initializeUsers(
	log *zap.SugaredLogger,
	dir DataDir,
	_db *gorm.DB,
) (err error) {
	buf, _ := os.ReadFile(filepath.Join(dir.String(), "users.yaml"))
	buf = bytes.TrimSpace(buf)
	if len(buf) == 0 {
		return
	}

	type InitialUser struct {
		Username       string `yaml:"username"`
		Password       string `yaml:"password"`
		IsAdmin        bool   `yaml:"is_admin"`
		UpdateExisting bool   `yaml:"update_existing"`
	}

	db := dao.Use(_db)

	dec := yaml.NewDecoder(bytes.NewReader(buf))

	for {
		var iu InitialUser
		if err = dec.Decode(&iu); err != nil {
			break
		}

		var user *model.User
		if user, err = db.User.Where(db.User.ID.Eq(iu.Username)).First(); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.With("username", iu.Username).Info("user created")
				user = &model.User{
					ID:        iu.Username,
					CreatedAt: time.Now(),
					VisitedAt: time.Now(),
					IsAdmin:   iu.IsAdmin,
				}
				user.SetPassword(iu.Password)

				if err = db.User.Create(user); err != nil {
					return
				}
			} else {
				return
			}
		} else if iu.UpdateExisting {
			log.With("username", iu.Username).Info("user updated")

			user.SetPassword(iu.Password)

			if _, err = db.User.Where(
				db.User.ID.Eq(iu.Username),
			).UpdateSimple(
				db.User.PasswordDigest.Value(user.PasswordDigest),
				db.User.IsAdmin.Value(iu.IsAdmin),
			); err != nil {
				return
			}
		}
	}

	if errors.Is(err, io.EOF) {
		err = nil
	}

	return
}

func createDatabase(dir DataDir) (db *gorm.DB, err error) {
	if db, err = gorm.Open(
		sqlite.Open(filepath.Join(dir.String(), "database.sqlite3")),
		&gorm.Config{},
	); err != nil {
		return
	}
	if err = db.AutoMigrate(model.All...); err != nil {
		return
	}
	return
}
