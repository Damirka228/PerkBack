package config

import "os"

func ApplyEnv(cfg *Config) {
	addr, ok := os.LookupEnv("BENEFIT_HTTP_ADDR")
	if ok {
		cfg.HTTP.Addr = addr
	}
	rulesAddr, ok := os.LookupEnv("BENEFIT_RULES_ADDR")
	if ok {
		cfg.RulesService.Addr = rulesAddr
	}
}
