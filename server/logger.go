package server

import (
	"bytes"
	"log"
	"os"
	"text/template"
	"time"
)

var (
	LogFile     *os.File
	LogTemplate *template.Template
)

type logItem struct {
	RequestIP string
	Time      time.Time
	X         int
	Y         int
	Z         int
}

const DefaultLogFormat = "{{.Time}}:{{.RequestIP}} —— Tile:{{.Z}}/{{.X}}/{{.Y}}"

func Log(item logItem) {
	if LogFile == nil {
		return
	}

	if item.Time.IsZero() {
		item.Time = time.Now()
	}

	var l string
	lbuf := bytes.NewBufferString(l)

	if err := LogTemplate.Execute(lbuf, item); err != nil {
		// Don't care about the error.
		log.Println("Error writing to log file", err)
		return
	}
	b := lbuf.Bytes()

	// If there is no new line, let's add it.
	if b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}

	// Don't care about the error.
	LogFile.Write(b)
}
