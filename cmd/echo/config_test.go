package main

import "testing"

func TestLoadConfigUsesDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("MAX_BODY_BYTES", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.Port != 8080 {
		t.Fatalf("PORT = %d, expected 8080", cfg.Port)
	}
}

func TestLoadCOnfigRefectsInvalidPorts(t *testing.T) {
	tests := []string{"Not a Number", "0", "65536", "-1"}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			t.Setenv("PORT", value)
			if _, err := loadConfig(); err == nil {
				t.Fatalf("loadConfig() succeeded for invalid PORT %q", value)
			}
		})
	}
}
