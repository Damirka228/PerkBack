package config

import "fmt"

func (c Config) Validate() error {
	if c.App.Name == "" {
		return fmt.Errorf("app name is empty")
	}
	if c.GRPC.Addr == "" {
		return fmt.Errorf("grpc addr is empty")
	}
	if c.HTTP.Addr == "" {
		return fmt.Errorf("http addr is empty")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout <= 0")
	}
	return nil
}
