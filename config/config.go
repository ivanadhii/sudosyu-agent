package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	BackendURL      string `yaml:"backend_url"`
	APIKey          string `yaml:"api_key"`
	ServerName      string `yaml:"server_name"`
	IntervalSeconds int    `yaml:"interval_seconds"`
	DockerSocket    string `yaml:"docker_socket"`
}

func Load(path string) (Config, error) {
	cfg := Config{
		BackendURL:      "http://localhost:8080",
		IntervalSeconds: 10,
		DockerSocket:    "/var/run/docker.sock",
	}

	if path != "" {
		f, err := os.Open(path)
		if err == nil {
			defer f.Close()
			_ = yaml.NewDecoder(f).Decode(&cfg)
		}
	}

	// Env overrides
	if v := os.Getenv("BACKEND_URL"); v != "" {
		cfg.BackendURL = v
	}
	if v := os.Getenv("API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("SERVER_NAME"); v != "" {
		cfg.ServerName = v
	}
	if v := os.Getenv("INTERVAL_SECONDS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.IntervalSeconds = i
		}
	}
	if v := os.Getenv("DOCKER_SOCKET"); v != "" {
		cfg.DockerSocket = v
	}

	return cfg, nil
}
