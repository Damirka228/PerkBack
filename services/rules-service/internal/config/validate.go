package config

import "fmt"

func (c Config) Validate() error {
	if c.App.Name == "" {
		return fmt.Errorf("app name is empty")
	}
	if c.GRPC.Address == "" {
		return fmt.Errorf("grpc addr is empty")
	}
	if c.HTTP.Address == "" {
		return fmt.Errorf("http addr is empty")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout <= 0")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout <= 0")
	}
	if c.DB.ConnectTimeout <= 0 {
		return fmt.Errorf("database timeout <= 0")
	}
	if c.DB.DSN == "" {
		return fmt.Errorf("database url is empty")
	}
	return nil
}
