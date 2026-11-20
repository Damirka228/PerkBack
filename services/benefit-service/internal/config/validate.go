package config

import (
	"fmt"
	"strings"
)

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
	if c.RulesService.Addr == "" {
		return fmt.Errorf("rules service address is empty")
	}
	if c.RulesService.RequestTimeout <= 0 {
		return fmt.Errorf("timeout rule service <= 0")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout <= 0")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout <= 0")
	}
	if len(c.KafkaConfig.Brokers) < 1 {
		return fmt.Errorf("at least one Kafka broker is required")
	}
	for _, broker := range c.KafkaConfig.Brokers {
		if strings.TrimSpace(broker) == "" {
			return fmt.Errorf("broker is empty")
		}
	}
	if strings.TrimSpace(c.KafkaConfig.Topic) == "" {
		return fmt.Errorf("kafka topic is empty")
	}
	if strings.TrimSpace(c.KafkaConfig.GroupId) == "" {
		return fmt.Errorf("kafka group id is empty")
	}
	return nil
}
