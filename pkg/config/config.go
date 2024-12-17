package config

import (
	"log"
	"os"
	"strconv"
)

// Config holds the configuration for the application.
type Config struct {
	Debug                 bool
	MetricsPort           int
	Namespace             string
	ExcludeNamespaceLabel string
	SyncInterval          int // Interval in minutes
}

// CFG holds the configuration for the application.
var CFG Config

// LoadConfigFromEnv loads the configuration from environment variables.
func LoadConfigFromEnv() Config {
	config := Config{
		Debug:                 parseEnvBool("DEBUG"),
		MetricsPort:           parseEnvInt("METRICS_PORT", 9090),
		Namespace:             getEnvOrDefault("NAMESPACE", ""),
		ExcludeNamespaceLabel: getEnvOrDefault("EXCLUDE_NAMESPACE_LABEL", ""),
		SyncInterval:          parseEnvInt("SYNC_INTERVAL", 15), // Default to 15 minutes
	}

	CFG = config

	return config
}

// getEnvOrDefault returns the value of the environment variable with the given key.
func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// parseEnvInt parses the value of the environment variable with the given key as an integer.
func parseEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("Failed to parse environment variable %s: %v. Using default value: %d", key, err, defaultValue)
		return defaultValue
	}
	return intValue
}

// parseEnvBool parses the value of the environment variable with the given key as a boolean.
func parseEnvBool(key string) bool {
	value := os.Getenv(key)
	return value == "true"
}
