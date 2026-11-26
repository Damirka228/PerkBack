package config

import "os"

func ApplyEnv(cfg *Config) {
	if dsn, ok := os.LookupEnv("USER_DATABASE_URL"); ok {
		cfg.DB.DSN = dsn
	}
}
