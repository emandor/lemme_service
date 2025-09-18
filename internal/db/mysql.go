package db

import (
	"embed"
	"log"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"
)

//go:embed migrations/*.sql
var fs embed.FS

func MustConnect(dsn string) *sqlx.DB {
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	return db
}

func MustMigrate(db *sqlx.DB) {
	d, _ := mysql.WithInstance(db.DB, &mysql.Config{})
	s, _ := iofs.New(fs, "migrations")
	m, err := migrate.NewWithInstance("iofs", s, "mysql", d)
	if err != nil {
		log.Fatal(err)
	}
	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatal(err)
	}
}
