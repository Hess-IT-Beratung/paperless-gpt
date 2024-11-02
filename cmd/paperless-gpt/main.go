package main

import (
	"paperless-gpt/internal/service"

	"github.com/sirupsen/logrus"
)

var (
	Log = logrus.New()
)

func init() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
}

func main() {
	service.Start()
}
