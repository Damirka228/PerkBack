package config

import "time"

type AppConfig struct {
	Name string `yaml:"name"`
	Env  string `yaml:"env"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

type GRPCConfig struct {
	Addr string `yaml:"address"`
}

type HTTPConfig struct {
	Addr string `yaml:"address"`
}

type Config struct {
	App             AppConfig     `yaml:"app"`
	GRPC            GRPCConfig    `yaml:"grpc"`
	HTTP            HTTPConfig    `yaml:"http"`
	Log             LogConfig     `yaml:"log"`
	Timeout         time.Duration `yaml:"timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}
