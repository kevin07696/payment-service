package main

import (
	"os"
	"testing"
)

// TestEnvRequire tests the envRequire function
func TestEnvRequire(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		envValue  string
		wantValue string
		wantErr   bool
	}{
		{
			name:      "returns value when set",
			key:       "TEST_ENV_VAR",
			envValue:  "test_value",
			wantValue: "test_value",
			wantErr:   false,
		},
		{
			name:      "returns error when empty",
			key:       "TEST_ENV_VAR",
			envValue:  "",
			wantValue: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			// Test
			got, err := envRequire(tt.key)

			// Verify
			if (err != nil) != tt.wantErr {
				t.Errorf("envRequire() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantValue {
				t.Errorf("envRequire() = %v, want %v", got, tt.wantValue)
			}
		})
	}
}

// TestEnvParseInt tests the envParseInt function
func TestEnvParseInt(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantValue int
		wantErr   bool
	}{
		{
			name:      "parses valid integer",
			value:     "42",
			wantValue: 42,
			wantErr:   false,
		},
		{
			name:      "parses zero",
			value:     "0",
			wantValue: 0,
			wantErr:   false,
		},
		{
			name:      "parses negative integer",
			value:     "-10",
			wantValue: -10,
			wantErr:   false,
		},
		{
			name:      "returns error for non-integer",
			value:     "abc",
			wantValue: 0,
			wantErr:   true,
		},
		{
			name:      "returns error for float",
			value:     "3.14",
			wantValue: 0,
			wantErr:   true,
		},
		{
			name:      "returns error for empty string",
			value:     "",
			wantValue: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := envParseInt(tt.value)

			if (err != nil) != tt.wantErr {
				t.Errorf("envParseInt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantValue {
				t.Errorf("envParseInt() = %v, want %v", got, tt.wantValue)
			}
		})
	}
}

// TestEnvParseInt32 tests the envParseInt32 function
func TestEnvParseInt32(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantValue int32
		wantErr   bool
	}{
		{
			name:      "parses valid int32",
			value:     "100",
			wantValue: 100,
			wantErr:   false,
		},
		{
			name:      "returns error for overflow",
			value:     "9999999999999999999",
			wantValue: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := envParseInt32(tt.value)

			if (err != nil) != tt.wantErr {
				t.Errorf("envParseInt32() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantValue {
				t.Errorf("envParseInt32() = %v, want %v", got, tt.wantValue)
			}
		})
	}
}

// TestEnvParseFloat tests the envParseFloat function
func TestEnvParseFloat(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantValue float64
		wantErr   bool
	}{
		{
			name:      "parses valid float",
			value:     "3.14",
			wantValue: 3.14,
			wantErr:   false,
		},
		{
			name:      "parses integer as float",
			value:     "42",
			wantValue: 42.0,
			wantErr:   false,
		},
		{
			name:      "parses negative float",
			value:     "-1.5",
			wantValue: -1.5,
			wantErr:   false,
		},
		{
			name:      "returns error for non-numeric",
			value:     "abc",
			wantValue: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := envParseFloat(tt.value)

			if (err != nil) != tt.wantErr {
				t.Errorf("envParseFloat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantValue {
				t.Errorf("envParseFloat() = %v, want %v", got, tt.wantValue)
			}
		})
	}
}

// TestEnvParseBool tests the envParseBool function
func TestEnvParseBool(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantValue bool
		wantErr   bool
	}{
		{
			name:      "parses 'true'",
			value:     "true",
			wantValue: true,
			wantErr:   false,
		},
		{
			name:      "parses 'TRUE' (case insensitive)",
			value:     "TRUE",
			wantValue: true,
			wantErr:   false,
		},
		{
			name:      "parses '1'",
			value:     "1",
			wantValue: true,
			wantErr:   false,
		},
		{
			name:      "parses 'false'",
			value:     "false",
			wantValue: false,
			wantErr:   false,
		},
		{
			name:      "parses 'FALSE' (case insensitive)",
			value:     "FALSE",
			wantValue: false,
			wantErr:   false,
		},
		{
			name:      "parses '0'",
			value:     "0",
			wantValue: false,
			wantErr:   false,
		},
		{
			name:      "returns error for 'yes'",
			value:     "yes",
			wantValue: false,
			wantErr:   true,
		},
		{
			name:      "returns error for 'no'",
			value:     "no",
			wantValue: false,
			wantErr:   true,
		},
		{
			name:      "returns error for empty string",
			value:     "",
			wantValue: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := envParseBool(tt.value)

			if (err != nil) != tt.wantErr {
				t.Errorf("envParseBool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantValue {
				t.Errorf("envParseBool() = %v, want %v", got, tt.wantValue)
			}
		})
	}
}

// TestEnvParseStringList tests the envParseStringList function
func TestEnvParseStringList(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantValue []string
	}{
		{
			name:      "parses comma-separated list",
			value:     "a,b,c",
			wantValue: []string{"a", "b", "c"},
		},
		{
			name:      "trims whitespace",
			value:     " a , b , c ",
			wantValue: []string{"a", "b", "c"},
		},
		{
			name:      "returns nil for empty string",
			value:     "",
			wantValue: nil,
		},
		{
			name:      "handles single item",
			value:     "single",
			wantValue: []string{"single"},
		},
		{
			name:      "skips empty items",
			value:     "a,,b,,,c",
			wantValue: []string{"a", "b", "c"},
		},
		{
			name:      "handles domains",
			value:     "example.com,app.example.com,mysite.org",
			wantValue: []string{"example.com", "app.example.com", "mysite.org"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := envParseStringList(tt.value)

			// Compare slices
			if len(got) != len(tt.wantValue) {
				t.Errorf("envParseStringList() = %v, want %v", got, tt.wantValue)
				return
			}
			for i := range got {
				if got[i] != tt.wantValue[i] {
					t.Errorf("envParseStringList() = %v, want %v", got, tt.wantValue)
					return
				}
			}
		})
	}
}

// TestConfigLoaderCollectsErrors tests that configLoader collects multiple errors
func TestConfigLoaderCollectsErrors(t *testing.T) {
	// Clear all environment variables that loadConfig requires
	requiredVars := []string{
		"PORT", "ENVIRONMENT", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD",
		"DB_NAME", "DB_SSL_MODE", "DB_MAX_CONNS", "DB_MIN_CONNS",
	}
	for _, v := range requiredVars {
		os.Unsetenv(v)
	}

	// Create config loader and try to load some values
	c := &configLoader{}

	// Try to load missing values
	_ = c.mustString("MISSING_VAR_1")
	_ = c.mustString("MISSING_VAR_2")
	_ = c.mustInt("MISSING_VAR_3")

	// Verify multiple errors were collected
	if !c.hasErrors() {
		t.Error("expected errors to be collected")
	}

	errs := c.errors()
	if len(errs) != 3 {
		t.Errorf("expected 3 errors, got %d", len(errs))
	}
}

// TestConfigLoaderMustInt tests mustInt with valid and invalid values
func TestConfigLoaderMustInt(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		wantValue int
		wantErr   bool
	}{
		{
			name:      "parses valid int",
			envValue:  "8080",
			wantValue: 8080,
			wantErr:   false,
		},
		{
			name:      "returns error for invalid int",
			envValue:  "not-a-number",
			wantValue: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_MUST_INT"
			os.Setenv(key, tt.envValue)
			defer os.Unsetenv(key)

			c := &configLoader{}
			got := c.mustInt(key)

			if (c.hasErrors()) != tt.wantErr {
				t.Errorf("mustInt() hasErrors = %v, wantErr %v", c.hasErrors(), tt.wantErr)
				return
			}
			if got != tt.wantValue {
				t.Errorf("mustInt() = %v, want %v", got, tt.wantValue)
			}
		})
	}
}

// TestConfigLoaderMustBool tests mustBool with valid and invalid values
func TestConfigLoaderMustBool(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		wantValue bool
		wantErr   bool
	}{
		{
			name:      "parses true",
			envValue:  "true",
			wantValue: true,
			wantErr:   false,
		},
		{
			name:      "parses false",
			envValue:  "false",
			wantValue: false,
			wantErr:   false,
		},
		{
			name:      "returns error for invalid bool",
			envValue:  "maybe",
			wantValue: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_MUST_BOOL"
			os.Setenv(key, tt.envValue)
			defer os.Unsetenv(key)

			c := &configLoader{}
			got := c.mustBool(key)

			if (c.hasErrors()) != tt.wantErr {
				t.Errorf("mustBool() hasErrors = %v, wantErr %v", c.hasErrors(), tt.wantErr)
				return
			}
			if got != tt.wantValue {
				t.Errorf("mustBool() = %v, want %v", got, tt.wantValue)
			}
		})
	}
}

// TestEnvStringDefault tests the envStringDefault helper
func TestEnvStringDefault(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		defaultValue string
		want         string
	}{
		{
			name:         "returns value when set",
			value:        "custom",
			defaultValue: "default",
			want:         "custom",
		},
		{
			name:         "returns default when empty",
			value:        "",
			defaultValue: "default",
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := envStringDefault(tt.value, tt.defaultValue)
			if got != tt.want {
				t.Errorf("envStringDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestEnvParseBoolDefault tests the envParseBoolDefault helper
func TestEnvParseBoolDefault(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		defaultValue bool
		want         bool
	}{
		{
			name:         "returns parsed true",
			value:        "true",
			defaultValue: false,
			want:         true,
		},
		{
			name:         "returns parsed false",
			value:        "false",
			defaultValue: true,
			want:         false,
		},
		{
			name:         "returns default when empty",
			value:        "",
			defaultValue: true,
			want:         true,
		},
		{
			name:         "returns default when invalid",
			value:        "invalid",
			defaultValue: true,
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := envParseBoolDefault(tt.value, tt.defaultValue)
			if got != tt.want {
				t.Errorf("envParseBoolDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestEnvParseFloatDefault tests the envParseFloatDefault helper
func TestEnvParseFloatDefault(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		defaultValue float64
		want         float64
	}{
		{
			name:         "returns parsed float",
			value:        "0.5",
			defaultValue: 0.1,
			want:         0.5,
		},
		{
			name:         "returns default when empty",
			value:        "",
			defaultValue: 0.1,
			want:         0.1,
		},
		{
			name:         "returns default when invalid",
			value:        "invalid",
			defaultValue: 0.1,
			want:         0.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := envParseFloatDefault(tt.value, tt.defaultValue)
			if got != tt.want {
				t.Errorf("envParseFloatDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}
