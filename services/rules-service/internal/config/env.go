package config

import "os"

func ApplyEnv(cfg *Config) {
	dbURL, ok := os.LookupEnv("RULES_DATABASE_URL")
	if ok {
		cfg.DB.DSN = dbURL
	}
}
