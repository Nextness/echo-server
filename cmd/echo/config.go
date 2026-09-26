package main

import (
	"fmt"
	"os"
	"strconv"
)

const (
	defaultPort = 8080
	// TODO: Should we make this public so that we don't have to repeat everywhere? Consider for later
	defaultMaxBodyBytes = int64(1 << 20)
)

type config struct {
	Port         int
	MaxBodyBytes int64
}

func loadConfig() (config, error) {
	cfg := config{
		Port:         defaultPort,
		MaxBodyBytes: defaultMaxBodyBytes,
	}

	if envVar := os.Getenv("PORT"); envVar != "" {
		port, err := strconv.Atoi(envVar)
		minPort, maxPort := 1, 65535
		if err != nil || port < minPort || port > maxPort {
			return config{}, fmt.Errorf("PORT must be an integer from %d to %d", minPort, maxPort)
		}
		cfg.Port = port
	}

	if envVar := os.Getenv("MAX_BODY_BYTES"); envVar != "" {
		limit, err := strconv.ParseInt(envVar, 10, 64)
		if err != nil || limit < 1 {
			return config{}, fmt.Errorf("MAX_BODY_BYTES must be a positive integer")
		}
		cfg.MaxBodyBytes = limit
	}

	return cfg, nil
}
