package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	"github.com/kipitix/growscada/internal/server/infrastructure/postgres/migrations"
	_ "github.com/lib/pq"
)

func main() {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = "postgres://growscada:growscada@localhost:5432/growscada?sslmode=disable"
	}
	runtimeDSN := os.Getenv("RUNTIME_DATABASE_DSN")
	if runtimeDSN == "" {
		runtimeDSN = dsn
	}
	engineeringDB, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer engineeringDB.Close()
	runtimeDB, err := sql.Open("postgres", runtimeDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer runtimeDB.Close()
	if err := migrations.Apply(context.Background(), engineeringDB, runtimeDB); err != nil {
		log.Fatal(err)
	}
}
