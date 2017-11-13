package util

import (
	"github.com/sirupsen/logrus"
	"os"
)

var CodeLogger *logrus.Logger

func init() {
	logFilename := "tegola.log"
	f, err := os.OpenFile(logFilename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
    if err != nil {
        logrus.Fatal(err)
    }
    CodeLogger = logrus.New()
    CodeLogger.Out = f
	CodeLogger.Level = logrus.DebugLevel
}
