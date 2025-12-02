package app

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	Port            int
	Debug           bool
	SecretKey       string
	DatabaseURL     string
	DefaultLanguage string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	// Load .env file if exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	config := &Config{
		Port:            getEnvInt("PORT", 8000),
		Debug:           getEnvBool("DEBUG", true),
		SecretKey:       getEnvStr("SECRET_KEY", "dev"),
		DatabaseURL:     getEnvStr("DATABASE_URL", "app.db"),
		DefaultLanguage: getEnvStr("DEFAULT_LANGUAGE", "en"),
	}

	// Warn about insecure secret key
	if config.SecretKey == "dev" {
		log.Println("WARNING: Using default SECRET_KEY='dev'. Set SECRET_KEY environment variable for production!")
	}

	return config
}

func getEnvStr(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}
