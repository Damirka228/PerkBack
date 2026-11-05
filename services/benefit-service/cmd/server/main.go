package main

import (
	"context"

	"github.com/sirupsen/logrus"
)

func main() {

	if err := run(context.Background(), "services/benefit-service/configs/local.yaml"); err != nil {
		logrus.WithError(err).Fatal("application failed")
	}
}
