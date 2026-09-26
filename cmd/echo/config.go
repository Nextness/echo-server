package main

import (
	"fmt"
	"os"
	"strconv"

	echoHandler "github.com/nextness/echo-server/internal/echo"
)

const defaultPort = 8080

type config struct {
	Port         int
	MaxBodyBytes int64
}

func loadConfig() (config, error) {
	cfg := config{
		Port:         defaultPort,
		MaxBodyBytes: echoHandler.DefaultMaxBodyBytes,
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
