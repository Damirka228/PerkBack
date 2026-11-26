package config

import (
	"fmt"
	"strings"
)

func (c Config) Validate() error {
	if strings.TrimSpace(c.App.Name) == "" {
		return fmt.Errorf("app name is empty")
	}
	if strings.TrimSpace(c.GRPC.Address) == "" {
		return fmt.Errorf("grpc address is empty")
	}
	if strings.TrimSpace(c.HTTP.Address) == "" {
		return fmt.Errorf("http address is empty")
	}
	if strings.TrimSpace(c.DB.DSN) == "" {
		return fmt.Errorf("USER_DATABASE_URL is required")
	}
	if c.DB.ConnectTimeout <= 0 {
		return fmt.Errorf("database connect timeout must be positive")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be positive")
	}
	return nil
}
