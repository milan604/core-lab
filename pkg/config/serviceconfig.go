package config

import (
	"encoding/json"
	"os"
)

type ServiceConfig struct {
	ServiceName string `json:"service_name"`
	ServiceCode string `json:"service_code"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Contact     string `json:"contact"`
	Repository  string `json:"repository"`
}

func LoadServiceConfig(path string) (*ServiceConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg ServiceConfig
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
