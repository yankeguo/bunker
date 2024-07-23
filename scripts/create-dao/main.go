package main

import (
	"log"
	"os"

	"github.com/glebarez/sqlite"
	"github.com/yankeguo/bunker/model"
	"github.com/yankeguo/rg"
	"gorm.io/gen"
	"gorm.io/gorm"
)

func main() {
	var err error
	defer func() {
		if err == nil {
			return
		}
		log.Println("exited with error:", err.Error())
		os.Exit(1)
	}()
	defer rg.Guard(&err)

	g := gen.NewGenerator(gen.Config{
		OutPath: "./model/dao",
		Mode:    gen.WithoutContext | gen.WithDefaultQuery,
	})

	db := rg.Must(gorm.Open(sqlite.Open("database.sqlite3"), &gorm.Config{})).Debug()

	rg.Must0(db.AutoMigrate(model.All...))

	g.UseDB(db)

	g.ApplyBasic(model.All...)

	g.Execute()
}
