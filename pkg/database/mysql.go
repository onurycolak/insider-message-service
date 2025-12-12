package database

import (
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/onurcolak/insider-message-service/environments"
	"github.com/onurcolak/insider-message-service/pkg/logger"
)

func NewMySQLDB(cfg environments.DatabaseConfig) (*sqlx.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName,
	)

	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Infof("Connected to MySQL database")
	return db, nil
}

func RunMigrations(db *sqlx.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		content TEXT NOT NULL,
		phone_number VARCHAR(20) NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		message_id VARCHAR(100),
		sent_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_messages_status (status),
		INDEX idx_messages_created_at (created_at),
		INDEX idx_messages_sent_at (sent_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	logger.Infof("Database migrations completed")

	return nil
}

func SeedTestData(db *sqlx.DB) error {
	var count int

	err := db.Get(&count, "SELECT COUNT(*) FROM messages")
	if err != nil {
		return err
	}

	if count > 0 {
		logger.Infof("Database already has %d messages, skipping seed", count)
		return nil
	}

	testMessages := []struct {
		content     string
		phoneNumber string
	}{
		{"Hello! This is a test message from Insider.", "+905551234567"},
		{"Your verification code is 123456", "+905559876543"},
		{"Welcome to our platform! We're excited to have you.", "+905551112233"},
		{"Your order has been shipped. Track it here.", "+905554445566"},
		{"Reminder: Your appointment is tomorrow at 10 AM", "+905557778899"},
		{"Special offer just for you! 20% off all products.", "+905552223344"},
		{"Your password has been successfully reset.", "+905556667788"},
		{"Thank you for your purchase! Order #12345", "+905553334455"},
		{"Don't forget to complete your profile.", "+905558889900"},
		{"New features available! Check out what's new.", "+905551239876"},
	}

	for _, msg := range testMessages {
		_, err := db.Exec(
			"INSERT INTO messages (content, phone_number, status) VALUES (?, ?, 'pending')",
			msg.content, msg.phoneNumber,
		)
		if err != nil {
			return fmt.Errorf("failed to seed test data: %w", err)
		}
	}

	logger.Infof("Seeded %d test messages", len(testMessages))
	return nil
}
