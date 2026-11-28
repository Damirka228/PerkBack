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

type UserService struct {
	Addr           string        `yaml:"address"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
}

type KafkaConfig struct {
	Brokers []string `yaml:"brokers"`
	Topic   string   `yaml:"topic"`
	GroupId string   `yaml:"group_id"`
}

type DB struct {
	DSN            string        `yaml:"-"`
	ConnectTimeout time.Duration `yaml:"connect_timeout"`
}

type Config struct {
	App                     AppConfig     `yaml:"app"`
	GRPC                    GRPCConfig    `yaml:"grpc"`
	HTTP                    HTTPConfig    `yaml:"http"`
	RulesService            RulesService  `yaml:"rules-service"`
	UserService             UserService   `yaml:"user-service"`
	KafkaConfig             KafkaConfig   `yaml:"kafka"`
	DB                      DB            `yaml:"database"`
	Log                     LogConfig     `yaml:"log"`
	SelectedCategoryRateBPS int64         `yaml:"selected_category_rate_bps"`
	Timeout                 time.Duration `yaml:"timeout"`
	ShutdownTimeout         time.Duration `yaml:"shutdown_timeout"`
}
