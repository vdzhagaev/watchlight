package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env          string     `yaml:"env" env:"APP_ENV" env-default:"local"`
	StoragePath  string     `yaml:"storage_path" env-required:"true" env:"STORAGE_PATH"`
	HTTPServer   HTTPServer `yaml:"http_server"`
	Monitoring   Monitoring `yaml:"monitoring"`
	SlackWebhook string     `yaml:"slack_webhook" env:"SLACK_WEBHOOK_URL"`
}

type HTTPServer struct {
	Address         string        `yaml:"address" env-default:"0.0.0.0:8080"`
	Timeout         time.Duration `yaml:"timeout" env-default:"4s"`
	IdleTimeout     time.Duration `yaml:"idle_timeout" env-default:"60s"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" env-default:"5s"`
}

type Monitoring struct {
	DefaultInterval time.Duration `yaml:"default_interval" env-default:"60s"`
	CheckTimeout    time.Duration `yaml:"check_timeout" env-default:"10s"`
}

func Load() (*Config, error) {
	configPath := os.Getenv("CONFIG_PATH")

	if configPath == "" {
		return nil, fmt.Errorf("CONFIG_PATH not set in environment")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist: %s", configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("cannot read config: %w", err)
	}

	return &cfg, nil
}
