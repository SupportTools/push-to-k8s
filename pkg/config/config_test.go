package config

import (
	"os"
	"testing"
)

// Helper function to set environment variables for testing
func setEnv(t *testing.T, key, value string) {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set environment variable %s: %v", key, err)
	}
}

// Helper function to unset environment variables for testing
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("failed to unset environment variable %s: %v", key, err)
	}
}

// TestGetEnvOrDefault tests the getEnvOrDefault function
func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		value        string
		defaultValue string
		expected     string
		shouldSet    bool
	}{
		{
			name:         "environment variable set",
			key:          "TEST_VAR",
			value:        "test-value",
			defaultValue: "default-value",
			expected:     "test-value",
			shouldSet:    true,
		},
		{
			name:         "environment variable not set",
			key:          "TEST_VAR_MISSING",
			defaultValue: "default-value",
			expected:     "default-value",
			shouldSet:    false,
		},
		{
			name:         "environment variable empty string",
			key:          "TEST_VAR_EMPTY",
			value:        "",
			defaultValue: "default-value",
			expected:     "default-value",
			shouldSet:    true,
		},
		{
			name:         "default value is empty string",
			key:          "TEST_VAR_DEFAULT_EMPTY",
			defaultValue: "",
			expected:     "",
			shouldSet:    false,
		},
		{
			name:         "both env and default are empty",
			key:          "TEST_VAR_BOTH_EMPTY",
			value:        "",
			defaultValue: "",
			expected:     "",
			shouldSet:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.shouldSet {
				setEnv(t, tt.key, tt.value)
				defer unsetEnv(t, tt.key)
			}

			// Test
			result := getEnvOrDefault(tt.key, tt.defaultValue)

			// Verify
			if result != tt.expected {
				t.Errorf("getEnvOrDefault(%q, %q) = %q, want %q", tt.key, tt.defaultValue, result, tt.expected)
			}
		})
	}
}

// TestParseEnvInt tests the parseEnvInt function
func TestParseEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		value        string
		defaultValue int
		expected     int
		shouldSet    bool
	}{
		{
			name:         "valid positive integer",
			key:          "TEST_INT_VALID",
			value:        "42",
			defaultValue: 10,
			expected:     42,
			shouldSet:    true,
		},
		{
			name:         "valid zero",
			key:          "TEST_INT_ZERO",
			value:        "0",
			defaultValue: 10,
			expected:     0,
			shouldSet:    true,
		},
		{
			name:         "valid negative integer",
			key:          "TEST_INT_NEGATIVE",
			value:        "-5",
			defaultValue: 10,
			expected:     -5,
			shouldSet:    true,
		},
		{
			name:         "invalid format - text",
			key:          "TEST_INT_INVALID_TEXT",
			value:        "not-a-number",
			defaultValue: 99,
			expected:     99,
			shouldSet:    true,
		},
		{
			name:         "invalid format - float",
			key:          "TEST_INT_INVALID_FLOAT",
			value:        "3.14",
			defaultValue: 99,
			expected:     99,
			shouldSet:    true,
		},
		{
			name:         "environment variable not set",
			key:          "TEST_INT_MISSING",
			defaultValue: 100,
			expected:     100,
			shouldSet:    false,
		},
		{
			name:         "empty string",
			key:          "TEST_INT_EMPTY",
			value:        "",
			defaultValue: 50,
			expected:     50,
			shouldSet:    true,
		},
		{
			name:         "very large number",
			key:          "TEST_INT_LARGE",
			value:        "2147483647",
			defaultValue: 10,
			expected:     2147483647,
			shouldSet:    true,
		},
		{
			name:         "number with spaces",
			key:          "TEST_INT_SPACES",
			value:        " 42 ",
			defaultValue: 10,
			expected:     10, // Should fail to parse and use default
			shouldSet:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.shouldSet {
				setEnv(t, tt.key, tt.value)
				defer unsetEnv(t, tt.key)
			}

			// Test
			result := parseEnvInt(tt.key, tt.defaultValue)

			// Verify
			if result != tt.expected {
				t.Errorf("parseEnvInt(%q, %d) = %d, want %d", tt.key, tt.defaultValue, result, tt.expected)
			}
		})
	}
}

// TestParseEnvBool tests the parseEnvBool function
func TestParseEnvBool(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     string
		expected  bool
		shouldSet bool
	}{
		{
			name:      "true string",
			key:       "TEST_BOOL_TRUE",
			value:     "true",
			expected:  true,
			shouldSet: true,
		},
		{
			name:      "false string",
			key:       "TEST_BOOL_FALSE",
			value:     "false",
			expected:  false,
			shouldSet: true,
		},
		{
			name:      "empty string",
			key:       "TEST_BOOL_EMPTY",
			value:     "",
			expected:  false,
			shouldSet: true,
		},
		{
			name:      "environment variable not set",
			key:       "TEST_BOOL_MISSING",
			expected:  false,
			shouldSet: false,
		},
		{
			name:      "uppercase TRUE",
			key:       "TEST_BOOL_UPPERCASE",
			value:     "TRUE",
			expected:  false, // Function is case-sensitive
			shouldSet: true,
		},
		{
			name:      "1 (numeric true)",
			key:       "TEST_BOOL_ONE",
			value:     "1",
			expected:  false, // Function only accepts "true" string
			shouldSet: true,
		},
		{
			name:      "0 (numeric false)",
			key:       "TEST_BOOL_ZERO",
			value:     "0",
			expected:  false,
			shouldSet: true,
		},
		{
			name:      "yes",
			key:       "TEST_BOOL_YES",
			value:     "yes",
			expected:  false, // Function only accepts "true" string
			shouldSet: true,
		},
		{
			name:      "no",
			key:       "TEST_BOOL_NO",
			value:     "no",
			expected:  false,
			shouldSet: true,
		},
		{
			name:      "true with spaces",
			key:       "TEST_BOOL_SPACES",
			value:     " true ",
			expected:  false, // Spaces make it not equal to "true"
			shouldSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.shouldSet {
				setEnv(t, tt.key, tt.value)
				defer unsetEnv(t, tt.key)
			}

			// Test
			result := parseEnvBool(tt.key)

			// Verify
			if result != tt.expected {
				t.Errorf("parseEnvBool(%q) = %v, want %v (value was %q)", tt.key, result, tt.expected, tt.value)
			}
		})
	}
}

// TestLoadConfigFromEnv tests the LoadConfigFromEnv function
func TestLoadConfigFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected Config
	}{
		{
			name:    "all defaults",
			envVars: map[string]string{},
			expected: Config{
				Debug:                 false,
				MetricsPort:           9090,
				Namespace:             "",
				ExcludeNamespaceLabel: "",
				SyncInterval:          15,
			},
		},
		{
			name: "all custom values",
			envVars: map[string]string{
				"DEBUG":                   "true",
				"METRICS_PORT":            "8080",
				"NAMESPACE":               "my-namespace",
				"EXCLUDE_NAMESPACE_LABEL": "no-sync",
				"SYNC_INTERVAL":           "30",
			},
			expected: Config{
				Debug:                 true,
				MetricsPort:           8080,
				Namespace:             "my-namespace",
				ExcludeNamespaceLabel: "no-sync",
				SyncInterval:          30,
			},
		},
		{
			name: "mixed values",
			envVars: map[string]string{
				"DEBUG":         "true",
				"NAMESPACE":     "test-ns",
				"SYNC_INTERVAL": "60",
			},
			expected: Config{
				Debug:                 true,
				MetricsPort:           9090, // default
				Namespace:             "test-ns",
				ExcludeNamespaceLabel: "", // default
				SyncInterval:          60,
			},
		},
		{
			name: "invalid metrics port uses default",
			envVars: map[string]string{
				"METRICS_PORT": "invalid",
				"NAMESPACE":    "test-ns",
			},
			expected: Config{
				Debug:                 false,
				MetricsPort:           9090, // default due to invalid value
				Namespace:             "test-ns",
				ExcludeNamespaceLabel: "",
				SyncInterval:          15,
			},
		},
		{
			name: "invalid sync interval uses default",
			envVars: map[string]string{
				"SYNC_INTERVAL": "not-a-number",
				"NAMESPACE":     "test-ns",
			},
			expected: Config{
				Debug:                 false,
				MetricsPort:           9090,
				Namespace:             "test-ns",
				ExcludeNamespaceLabel: "",
				SyncInterval:          15, // default due to invalid value
			},
		},
		{
			name: "debug false",
			envVars: map[string]string{
				"DEBUG":     "false",
				"NAMESPACE": "test-ns",
			},
			expected: Config{
				Debug:                 false,
				MetricsPort:           9090,
				Namespace:             "test-ns",
				ExcludeNamespaceLabel: "",
				SyncInterval:          15,
			},
		},
		{
			name: "zero metrics port uses default (invalid)",
			envVars: map[string]string{
				"METRICS_PORT": "0",
				"NAMESPACE":    "test-ns",
			},
			expected: Config{
				Debug:                 false,
				MetricsPort:           9090, // Zero is invalid, use default
				Namespace:             "test-ns",
				ExcludeNamespaceLabel: "",
				SyncInterval:          15,
			},
		},
		{
			name: "negative metrics port uses default",
			envVars: map[string]string{
				"METRICS_PORT": "-8080",
				"NAMESPACE":    "test-ns",
			},
			expected: Config{
				Debug:                 false,
				MetricsPort:           9090, // Negative is invalid, use default
				Namespace:             "test-ns",
				ExcludeNamespaceLabel: "",
				SyncInterval:          15,
			},
		},
		{
			name: "metrics port too large uses default",
			envVars: map[string]string{
				"METRICS_PORT": "70000",
				"NAMESPACE":    "test-ns",
			},
			expected: Config{
				Debug:                 false,
				MetricsPort:           9090, // > 65535 is invalid, use default
				Namespace:             "test-ns",
				ExcludeNamespaceLabel: "",
				SyncInterval:          15,
			},
		},
		{
			name: "valid edge case metrics port 1",
			envVars: map[string]string{
				"METRICS_PORT": "1",
				"NAMESPACE":    "test-ns",
			},
			expected: Config{
				Debug:                 false,
				MetricsPort:           1, // 1 is valid minimum
				Namespace:             "test-ns",
				ExcludeNamespaceLabel: "",
				SyncInterval:          15,
			},
		},
		{
			name: "valid edge case metrics port 65535",
			envVars: map[string]string{
				"METRICS_PORT": "65535",
				"NAMESPACE":    "test-ns",
			},
			expected: Config{
				Debug:                 false,
				MetricsPort:           65535, // 65535 is valid maximum
				Namespace:             "test-ns",
				ExcludeNamespaceLabel: "",
				SyncInterval:          15,
			},
		},
		{
			name: "zero sync interval uses default (invalid)",
			envVars: map[string]string{
				"SYNC_INTERVAL": "0",
				"NAMESPACE":     "test-ns",
			},
			expected: Config{
				Debug:                 false,
				MetricsPort:           9090,
				Namespace:             "test-ns",
				ExcludeNamespaceLabel: "",
				SyncInterval:          15, // Zero is invalid, use default
			},
		},
		{
			name: "negative sync interval uses default",
			envVars: map[string]string{
				"SYNC_INTERVAL": "-10",
				"NAMESPACE":     "test-ns",
			},
			expected: Config{
				Debug:                 false,
				MetricsPort:           9090,
				Namespace:             "test-ns",
				ExcludeNamespaceLabel: "",
				SyncInterval:          15, // Negative is invalid, use default
			},
		},
		{
			name: "sync interval too large uses default",
			envVars: map[string]string{
				"SYNC_INTERVAL": "2000",
				"NAMESPACE":     "test-ns",
			},
			expected: Config{
				Debug:                 false,
				MetricsPort:           9090,
				Namespace:             "test-ns",
				ExcludeNamespaceLabel: "",
				SyncInterval:          15, // > 1440 is invalid, use default
			},
		},
		{
			name: "valid edge case sync interval 1",
			envVars: map[string]string{
				"SYNC_INTERVAL": "1",
				"NAMESPACE":     "test-ns",
			},
			expected: Config{
				Debug:                 false,
				MetricsPort:           9090,
				Namespace:             "test-ns",
				ExcludeNamespaceLabel: "",
				SyncInterval:          1, // 1 is valid minimum
			},
		},
		{
			name: "valid edge case sync interval 1440",
			envVars: map[string]string{
				"SYNC_INTERVAL": "1440",
				"NAMESPACE":     "test-ns",
			},
			expected: Config{
				Debug:                 false,
				MetricsPort:           9090,
				Namespace:             "test-ns",
				ExcludeNamespaceLabel: "",
				SyncInterval:          1440, // 1440 is valid maximum
			},
		},
		{
			name: "empty namespace",
			envVars: map[string]string{
				"NAMESPACE": "",
			},
			expected: Config{
				Debug:                 false,
				MetricsPort:           9090,
				Namespace:             "",
				ExcludeNamespaceLabel: "",
				SyncInterval:          15,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup - Clear all environment variables first
			envVarsToClean := []string{"DEBUG", "METRICS_PORT", "NAMESPACE", "EXCLUDE_NAMESPACE_LABEL", "SYNC_INTERVAL"}
			for _, key := range envVarsToClean {
				unsetEnv(t, key)
			}

			// Set test environment variables
			for key, value := range tt.envVars {
				setEnv(t, key, value)
			}
			defer func() {
				// Cleanup
				for key := range tt.envVars {
					unsetEnv(t, key)
				}
			}()

			// Test
			result := LoadConfigFromEnv()

			// Verify
			if result.Debug != tt.expected.Debug {
				t.Errorf("Debug = %v, want %v", result.Debug, tt.expected.Debug)
			}
			if result.MetricsPort != tt.expected.MetricsPort {
				t.Errorf("MetricsPort = %d, want %d", result.MetricsPort, tt.expected.MetricsPort)
			}
			if result.Namespace != tt.expected.Namespace {
				t.Errorf("Namespace = %q, want %q", result.Namespace, tt.expected.Namespace)
			}
			if result.ExcludeNamespaceLabel != tt.expected.ExcludeNamespaceLabel {
				t.Errorf("ExcludeNamespaceLabel = %q, want %q", result.ExcludeNamespaceLabel, tt.expected.ExcludeNamespaceLabel)
			}
			if result.SyncInterval != tt.expected.SyncInterval {
				t.Errorf("SyncInterval = %d, want %d", result.SyncInterval, tt.expected.SyncInterval)
			}
		})
	}
}

