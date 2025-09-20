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
		log.Fatal("db connect error:", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	return db
}

func MustMigrate(db *sqlx.DB) {
	driver, err := mysql.WithInstance(db.DB, &mysql.Config{})
	if err != nil {
		log.Fatal("mysql driver init error:", err)
	}

	source, err := iofs.New(fs, "migrations")
	if err != nil {
		log.Fatal("migration source init error:", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "mysql", driver)
	if err != nil {
		log.Fatal("migration init error:", err)
	}

	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			log.Println("no new migrations")
			return
		}
		log.Fatal("migration failed:", err)
	}

	log.Println("migrations applied successfully")
}
