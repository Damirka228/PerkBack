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

type RulesService struct {
	Addr           string        `yaml:"address"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
}

type Config struct {
	App             AppConfig     `yaml:"app"`
	GRPC            GRPCConfig    `yaml:"grpc"`
	HTTP            HTTPConfig    `yaml:"http"`
	RulesService    RulesService  `yaml:"rules-service"`
	Log             LogConfig     `yaml:"log"`
	Timeout         time.Duration `yaml:"timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}
