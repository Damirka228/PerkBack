package config

import (
	"bytes"
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("err read config: %s:%w", path, err)
	}

	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	err = decoder.Decode(&cfg)
	if err != nil {
		return Config{}, fmt.Errorf("err decode config: %s:%w", path, err)
	}
	return cfg, nil

}
