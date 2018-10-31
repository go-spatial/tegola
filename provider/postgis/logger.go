package postgis

import (
	"log"

	"github.com/jackc/pgx"
)

func NewLogger() *Logger {
	return &Logger{}
}

type Logger struct{}

func (l Logger) Log(level pgx.LogLevel, msg string, data map[string]interface{}) {
	log.Printf("[%v] %v - %+v", level, msg, data)
}
