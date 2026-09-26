package main

import (
	"testing"

	echoHandler "github.com/nextness/echo-server/internal/echo"
)

func TestLoadConfigUsesDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("MAX_BODY_BYTES", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("Got loadConfig() error = %v, but expected nil", err)
	}

	expectedPort := 8080
	if cfg.Port != expectedPort {
		t.Fatalf("Got PORT = %d, but expected %d", cfg.Port, expectedPort)
	}
	expectedMaxBodyBytes := echoHandler.DefaultMaxBodyBytes
	if cfg.MaxBodyBytes != expectedMaxBodyBytes {
		t.Fatalf("Got MAX_BODY_BYTES = %d, but expected %d", cfg.MaxBodyBytes, expectedMaxBodyBytes)
	}
}

func TestLoadConfigAcceptsCustomValues(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("MAX_BODY_BYTES", "2048")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("Got loadConfig() error = %v, but expected nil", err)
	}
	expectedPort := 9090
	if cfg.Port != expectedPort {
		t.Errorf("Got PORT = %d, but expected %d", cfg.Port, expectedPort)
	}
	expectedMaxBodyBytes := int64(2048)
	if cfg.MaxBodyBytes != expectedMaxBodyBytes {
		t.Errorf("Got MAX_BODY_BYTES = %d, but expected %d", cfg.MaxBodyBytes, expectedMaxBodyBytes)
	}
}

func TestLoadConfigAcceptsPortBoundaries(t *testing.T) {
	type localStruct struct {
		value        string
		expectedPort int
	}
	tests := []localStruct{
		{value: "1", expectedPort: 1},
		{value: "65535", expectedPort: 65535},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			t.Setenv("PORT", test.value)
			t.Setenv("MAX_BODY_BYTES", "")
			cfg, err := loadConfig()
			if err != nil {
				t.Fatalf("Got loadConfig() error = %v, but expected nil", err)
			}
			if cfg.Port != test.expectedPort {
				t.Errorf("Got PORT = %d, but expected %d", cfg.Port, test.expectedPort)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidPorts(t *testing.T) {
	tests := []string{"not-a-number", "0", "65536", "-1", "9223372036854775808"}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			t.Setenv("PORT", value)
			t.Setenv("MAX_BODY_BYTES", "")
			if _, err := loadConfig(); err == nil {
				t.Fatalf("Got nil error for invalid PORT %q, but expected an error", value)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidBodyLimits(t *testing.T) {
	tests := []string{"not-a-number", "0", "-1", "9223372036854775808"}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			t.Setenv("PORT", "")
			t.Setenv("MAX_BODY_BYTES", value)
			if _, err := loadConfig(); err == nil {
				t.Fatalf("Got nil error for invalid MAX_BODY_BYTES %q, but expected an error", value)
			}
		})
	}
}
