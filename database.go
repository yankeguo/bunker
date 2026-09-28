package bunker

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/libtnb/sqlite"
	"github.com/yankeguo/bunker/model"
	"github.com/yankeguo/bunker/model/dao"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

func InitializeUsers(
	log *slog.Logger,
	dir DataDir,
	_db *gorm.DB,
) (err error) {
	var buf []byte
	if buf, err = os.ReadFile(filepath.Join(dir.String(), "users.yaml")); err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return
	}

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

		if iu.Username == "" || iu.Password == "" {
			log.With("username", iu.Username).Warn("skipped initial user with empty username or password")
			continue
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
				if err = user.SetPassword(iu.Password); err != nil {
					return
				}

				if err = db.User.Create(user); err != nil {
					return
				}
			} else {
				return
			}
		} else if iu.UpdateExisting {
			log.With("username", iu.Username).Info("user updated")

			passwordChanged := !user.CheckPassword(iu.Password)

			if err = user.SetPassword(iu.Password); err != nil {
				return
			}

			// password change and session revocation commit together, so a
			// failure cannot leave the new password with the old sessions
			err = db.Transaction(func(tx *dao.Query) error {
				if _, err := tx.User.Where(tx.User.ID.Eq(iu.Username)).UpdateSimple(
					tx.User.PasswordDigest.Value(user.PasswordDigest),
					tx.User.IsAdmin.Value(iu.IsAdmin),
				); err != nil {
					return err
				}
				if !passwordChanged {
					return nil
				}
				_, err := tx.Token.Where(tx.Token.UserID.Eq(iu.Username)).Delete()
				return err
			})
			if err != nil {
				return
			}
		}
	}

	if errors.Is(err, io.EOF) {
		err = nil
	}

	return
}

func CreateDatabase(dir DataDir) (db *gorm.DB, err error) {
	if dir.String() != "" {
		if err = os.MkdirAll(dir.String(), 0o755); err != nil {
			return
		}
	}

	dsn := sqliteDSN(filepath.Join(dir.String(), "database.sqlite3"))

	if db, err = gorm.Open(
		sqlite.Open(dsn),
		&gorm.Config{},
	); err != nil {
		return
	}
	if err = db.AutoMigrate(model.All...); err != nil {
		return
	}
	if Debug("db") {
		db = db.Debug()
	}
	return
}

// sqliteDSN opens SQLite in WAL mode. Transactions take a reserved lock
// immediately (BEGIN IMMEDIATE) so check-then-update work, such as refusing
// to remove the last admin, cannot interleave.
func sqliteDSN(path string) string {
	return "file:" + path + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_txlock=immediate"
}

func CloseDatabase(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
