package environments

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Webhook  WebhookConfig
	Message  MessageConfig
	Alert    AlertConfig
	Auth     AuthConfig
}

type ServerConfig struct {
	Port string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type WebhookConfig struct {
	URL     string
	AuthKey string
	Timeout time.Duration
}

type MessageConfig struct {
	BatchSize        int
	SendInterval     time.Duration
	MaxContentLength int
}

type AlertConfig struct {
	WebhookURL     string
	IterationCount int
}

type AuthConfig struct {
	MessagesAPIKey  string
	SchedulerAPIKey string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: GetEnv("SERVER_PORT", "8080"),
		},
		Database: DatabaseConfig{
			Host:     GetEnv("DB_HOST", "localhost"),
			Port:     GetEnv("DB_PORT", "3306"),
			User:     GetEnv("DB_USER", "insider"),
			Password: GetEnv("DB_PASSWORD", "insider123"),
			DBName:   GetEnv("DB_NAME", "insider_messages"),
		},
		Redis: RedisConfig{
			Host:     GetEnv("REDIS_HOST", "localhost"),
			Port:     GetEnv("REDIS_PORT", "6379"),
			Password: GetEnv("REDIS_PASSWORD", ""),
			DB:       GetEnvAsInt("REDIS_DB", 0),
		},
		Webhook: WebhookConfig{
			URL:     GetEnv("WEBHOOK_URL", "https://webhook.site/your-unique-id"),
			AuthKey: GetEnv("WEBHOOK_AUTH_KEY", ""),
			Timeout: time.Duration(GetEnvAsInt("WEBHOOK_TIMEOUT_SECONDS", 30)) * time.Second,
		},
		Message: MessageConfig{
			BatchSize:        GetEnvAsInt("MESSAGE_BATCH_SIZE", 2),
			SendInterval:     time.Duration(GetEnvAsInt("MESSAGE_SEND_INTERVAL_MINUTES", 2)) * time.Minute,
			MaxContentLength: GetEnvAsInt("MESSAGE_MAX_CONTENT_LENGTH", 1000),
		},
		Alert: AlertConfig{
			WebhookURL:     GetEnv("ALERT_WEBHOOK_URL", ""),
			IterationCount: GetEnvAsInt("ALERT_ITERATION_COUNT", 0),
		},
		Auth: AuthConfig{
			MessagesAPIKey:  GetEnv("MESSAGES_API_KEY", ""),
			SchedulerAPIKey: GetEnv("SCHEDULER_API_KEY", ""),
		},
	}
}

func GetEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func GetEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func GetEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func GetEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
