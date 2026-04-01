package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL string
	APIAddr     string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		APIAddr:     os.Getenv("API_ADDR"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.APIAddr == "" {
		cfg.APIAddr = ":8080"
	}

	return cfg, nil
}
