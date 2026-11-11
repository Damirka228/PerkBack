package config

import "time"

type AppConfig struct {
	Name string `yaml:"name"`
	Env  string `yaml:"env"`
}

type GRPCConfig struct {
	Address string `yaml:"address"`
}

type HTTPConfig struct {
	Address string `yaml:"address"`
}

type DB struct {
	ConnectTimeout time.Duration `yaml:"connect_timeout"`
	DSN            string        `yaml:"-"`
}

type Log struct {
	Level string `yaml:"level"`
}

type Config struct {
	App             AppConfig     `yaml:"app"`
	GRPC            GRPCConfig    `yaml:"grpc"`
	HTTP            HTTPConfig    `yaml:"http"`
	DB              DB            `yaml:"database"`
	Log             Log           `yaml:"log"`
	Timeout         time.Duration `yaml:"timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}
