package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Storage StorageConfig `yaml:"storage"`
	Upload  UploadConfig  `yaml:"upload"`
	Naming  NamingConfig  `yaml:"naming"`
}

type ServerConfig struct {
	Host   string `yaml:"host"`
	Port   int    `yaml:"port"`
	ApiKey string `yaml:"api_key"`
}

type StorageConfig struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Bucket    string `yaml:"bucket"`
	UseSSL    bool   `yaml:"use_ssl"`
	Region    string `yaml:"region"`
}

type UploadConfig struct {
	AllowedExtensions []string `yaml:"allowed_extensions"`
	MaxFileSizeBytes  int64    `yaml:"max_file_size_bytes"`
	MaxBatchFiles     int      `yaml:"max_batch_files"`
}

type NamingConfig struct {
	Prefix string `yaml:"prefix"`
}

func LoadConfigFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config yaml: %w", err)
	}
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func ValidateConfig(cfg *Config) error {
	if cfg.Server.Host == "" {
		return fmt.Errorf("server.host is required")
	}
	if cfg.Server.Port <= 0 {
		return fmt.Errorf("server.port must be positive")
	}
	if cfg.Server.ApiKey == "" {
		return fmt.Errorf("server.api_key is required")
	}
	if cfg.Storage.Endpoint == "" {
		return fmt.Errorf("storage.endpoint is required")
	}
	if cfg.Storage.AccessKey == "" {
		return fmt.Errorf("storage.access_key is required")
	}
	if cfg.Storage.SecretKey == "" {
		return fmt.Errorf("storage.secret_key is required")
	}
	if cfg.Storage.Bucket == "" {
		return fmt.Errorf("storage.bucket is required")
	}
	if cfg.Storage.Region == "" {
		return fmt.Errorf("storage.region is required")
	}
	if cfg.Naming.Prefix == "" {
		return fmt.Errorf("naming.prefix is required")
	}
	if len(cfg.Upload.AllowedExtensions) == 0 {
		return fmt.Errorf("upload.allowed_extensions is required")
	}
	if cfg.Upload.MaxFileSizeBytes <= 0 {
		return fmt.Errorf("upload.max_file_size_bytes must be positive")
	}
	if cfg.Upload.MaxBatchFiles <= 0 {
		return fmt.Errorf("upload.max_batch_files must be positive")
	}
	return nil
}

func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}
