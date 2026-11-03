package main

import (
	"github.com/sirupsen/logrus"
)

func main() {

	if err := run("services/benefit-service/configs/local.yaml"); err != nil {
		logrus.WithError(err).Fatal("application startup failed")
	}
}
