package config

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewConfig(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected *Config
	}{
		{
			name: "default config",
			envVars: map[string]string{
				"PROJECT_ID": "test-project",
			},
			expected: &Config{
				ProjectID:  "test-project",
				LogLevel:   "info",
				ServerPort: "8080",
			},
		},
		{
			name: "custom config",
			envVars: map[string]string{
				"PROJECT_ID":  "custom-project",
				"LOG_LEVEL":   "debug",
				"SERVER_PORT": "9000",
			},
			expected: &Config{
				ProjectID:  "custom-project",
				LogLevel:   "debug",
				ServerPort: "9000",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment variables
			for key := range tt.envVars {
				os.Unsetenv(key)
			}

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			config := NewConfig()
			assert.Equal(t, tt.expected.ProjectID, config.ProjectID)
			assert.Equal(t, tt.expected.LogLevel, config.LogLevel)
			assert.Equal(t, tt.expected.ServerPort, config.ServerPort)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				ProjectID: "test-project",
			},
			wantErr: false,
		},
		{
			name: "missing project id",
			config: &Config{
				ProjectID: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewAppConfig(t *testing.T) {
	// Set test environment variables
	os.Setenv("PROJECT_ID", "test-project")
	defer os.Unsetenv("PROJECT_ID")

	// This test requires a real datastore client, so we'll skip it in CI
	if os.Getenv("CI") != "" {
		t.Skip("Skipping integration test in CI")
		return
	}

	// Note: This test would require actual GCP credentials
	// In a real scenario, you'd mock the datastore client
	t.Skip("Requires GCP credentials - implement with mocks for full unit testing")
}

func TestGetEnv(t *testing.T) {
	// Test with existing env var
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")

	result := getEnv("TEST_VAR", "default")
	assert.Equal(t, "test_value", result)

	// Test with non-existing env var
	result = getEnv("NON_EXISTING_VAR", "default")
	assert.Equal(t, "default", result)
}

func TestServicesClose(t *testing.T) {
	logger := logrus.New()

	// Create a mock datastore client (this would need to be mocked properly)
	services := &Services{
		Logger: logger,
		// Note: In real tests, you'd mock these dependencies
	}

	// Test that Close doesn't panic
	assert.NotPanics(t, func() {
		services.Close()
	}, "Close should not panic")
}
