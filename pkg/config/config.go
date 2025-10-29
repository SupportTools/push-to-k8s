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
	SyncInterval          int  // Interval in minutes
	SecretSyncDebounce    int  // Debounce window in seconds for batching secret changes
	SecretSyncRateLimit   int  // Rate limit for sync operations (ops per second)
	EnableSecretWatcher   bool // Enable/disable secret watcher
}

// LoadConfigFromEnv loads the configuration from environment variables.
func LoadConfigFromEnv() Config {
	metricsPort := parseEnvInt("METRICS_PORT", 9090)
	syncInterval := parseEnvInt("SYNC_INTERVAL", 15) // Default to 15 minutes
	secretSyncDebounce := parseEnvInt("SECRET_SYNC_DEBOUNCE_SECONDS", 5)
	secretSyncRateLimit := parseEnvInt("SECRET_SYNC_RATE_LIMIT", 10)

	// Validate MetricsPort range (1-65535)
	if metricsPort < 1 || metricsPort > 65535 {
		log.Printf("WARNING: METRICS_PORT value %d is out of valid range (1-65535). Using default value: 9090", metricsPort)
		metricsPort = 9090
	}

	// Validate SyncInterval range (1-1440 minutes = 24 hours)
	if syncInterval < 1 || syncInterval > 1440 {
		log.Printf("WARNING: SYNC_INTERVAL value %d is out of valid range (1-1440 minutes). Using default value: 15", syncInterval)
		syncInterval = 15
	}

	// Validate SecretSyncDebounce range (1-60 seconds)
	if secretSyncDebounce < 1 || secretSyncDebounce > 60 {
		log.Printf("WARNING: SECRET_SYNC_DEBOUNCE_SECONDS value %d is out of valid range (1-60 seconds). Using default value: 5", secretSyncDebounce)
		secretSyncDebounce = 5
	}

	// Validate SecretSyncRateLimit range (1-100 ops per second)
	if secretSyncRateLimit < 1 || secretSyncRateLimit > 100 {
		log.Printf("WARNING: SECRET_SYNC_RATE_LIMIT value %d is out of valid range (1-100 ops/sec). Using default value: 10", secretSyncRateLimit)
		secretSyncRateLimit = 10
	}

	config := Config{
		Debug:                 parseEnvBool("DEBUG"),
		MetricsPort:           metricsPort,
		Namespace:             getEnvOrDefault("NAMESPACE", ""),
		ExcludeNamespaceLabel: getEnvOrDefault("EXCLUDE_NAMESPACE_LABEL", ""),
		SyncInterval:          syncInterval,
		SecretSyncDebounce:    secretSyncDebounce,
		SecretSyncRateLimit:   secretSyncRateLimit,
		EnableSecretWatcher:   parseEnvBoolWithDefault("ENABLE_SECRET_WATCHER", true),
	}

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

// parseEnvBoolWithDefault parses the value of the environment variable with the given key as a boolean with a default value.
func parseEnvBoolWithDefault(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value == "true"
}
