package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL         string
	APIAddr             string
	NATSURL             string
	OpsDockerEnabled    bool
	OpsDockerSocketPath string
	OpsContainerPrefix  string
	OpsDataPath         string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		APIAddr:             os.Getenv("API_ADDR"),
		NATSURL:             os.Getenv("NATS_URL"),
		OpsDockerEnabled:    isTruthy(os.Getenv("OPS_DOCKER_ENABLED")),
		OpsDockerSocketPath: os.Getenv("OPS_DOCKER_SOCKET"),
		OpsContainerPrefix:  os.Getenv("OPS_CONTAINER_PREFIX"),
		OpsDataPath:         os.Getenv("OPS_DATA_PATH"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.APIAddr == "" {
		cfg.APIAddr = ":8080"
	}
	if cfg.OpsDockerSocketPath == "" {
		cfg.OpsDockerSocketPath = "/var/run/docker.sock"
	}
	if cfg.OpsContainerPrefix == "" {
		cfg.OpsContainerPrefix = "solana-dashboard-"
	}
	if cfg.OpsDataPath == "" {
		cfg.OpsDataPath = "/ops-data"
	}

	return cfg, nil
}

func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
