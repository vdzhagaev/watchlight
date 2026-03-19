package config

import (
	"log"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env          string     `yaml:"env" env:"APP_ENV" env-default:"local"`
	Database     Database   `yaml:"database"`
	StoragePath  string     `yaml:"storage_path" env-required:"true" env:"STORAGE_PATH"`
	HTTPServer   HTTPServer `yaml:"http_server"`
	Monitoring   Monitoring `yaml:"monitoring"`
	SlackWebhook string     `yaml:"slack_webhook" env:"SLACK_WEBHOOK_URL"`
}

type Database struct {
	Host     string `yaml:"host" env:"DB_HOST" env-default:"localhost"`
	Port     string `yaml:"port" env:"DB_PORT" env-default:"5432"`
	User     string `yaml:"user" env:"DB_USER" env-default:"postgres"`
	Password string `yaml:"password" env:"DB_PASSWORD"`
	Name     string `yaml:"name" env:"DB_NAME" env-default:"uptime_db"`
	SSLMode  string `yaml:"sslmode" env:"DB_SSLMODE" env-default:"disable"`
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

func MustLoad() *Config {
	configPath := os.Getenv("CONFIG_PATH")

	if configPath == "" {
		log.Fatal("CONFIG_PATH not set in environment")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("config file does not exist: %s", configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("cannot read config: %s", err)
	}

	return &cfg
}

func (cfg *Config) DBConnString() string {
	return "host=" + cfg.Database.Host +
		" port=" + cfg.Database.Port +
		" user=" + cfg.Database.User +
		" password=" + cfg.Database.Password +
		" dbname=" + cfg.Database.Name +
		" sslmode=" + cfg.Database.SSLMode
}
