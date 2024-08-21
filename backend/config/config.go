package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	RedisAddr    string
	RedisDB      int
	ServerPort   string
	AuthUsername string
	AuthPassword string
}

func LoadConfig(filename string) (*Config, error) {
	// Load the environment file
	err := godotenv.Load(filename)
	if err != nil {
		return nil, fmt.Errorf("error loading .env file: %v", err)
	}

	// Initialize the Config struct with default values
	cfg := &Config{
		RedisAddr:    getEnv("REDIS_ADDR", ""),
		RedisDB:      getEnvInt("REDIS_DB", 0),
		ServerPort:   getEnv("SERVER_PORT", "8080"),
		AuthUsername: getEnv("AUTH_USERNAME", ""),
		AuthPassword: getEnv("AUTH_PASSWORD", ""),
	}

	// Validate required configurations
	if cfg.RedisAddr == "" {
		return nil, fmt.Errorf("REDIS_ADDR is required")
	}
	if cfg.ServerPort == "" {
		return nil, fmt.Errorf("SERVER_PORT is required")
	}

	return cfg, nil
}

// getEnv retrieves the value of the environment variable named by the key.
// It returns the value, which will be the default value if the variable is not present.
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvInt retrieves the value of the environment variable named by the key as an integer.
// It returns the value, which will be the default value if the variable is not present.
func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intValue
}
