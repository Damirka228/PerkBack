package logger

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

func NewLogger(level string) (*logrus.Logger, error) {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("error read log level: %w", err)
	}
	logger := logrus.New()
	logger.SetReportCaller(true)
	logger.SetLevel(lvl)
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		PrettyPrint:     true,
	})
	logger.SetOutput(os.Stdout)
	return logger, nil
}
