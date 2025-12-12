package main

import (
	"log"

	"github.com/onurcolak/insider-message-service/environments"
	"github.com/onurcolak/insider-message-service/pkg/database"
)

func main() {
	cfg := environments.Load()

	db, err := database.NewMySQLDB(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database: %v", err)
		}
	}()

	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	if err := database.SeedTestData(db); err != nil {
		log.Fatalf("Failed to seed test data: %v", err)
	}

	log.Println("Seed completed successfully")
}
