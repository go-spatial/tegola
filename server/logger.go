package server

import (
	"bytes"
	"log"
	"os"
	"sync"
	"text/template"
	"time"
)

type Logger struct {
	File   *os.File
	Format string
	sync.Mutex
	template *template.Template
	skip     bool
}

var L *Logger

type logItem struct {
	RequestIP string
	Time      time.Time
	X         int
	Y         int
	Z         int
}

const DefaultLogFormat = "{{.Time}}:{{.RequestIP}} —— Tile:{{.Z}}/{{.X}}/{{.Y}}"

func (l *Logger) initTemplate() {
	if l == nil || l.template != nil || l.skip {
		return
	}
	if l.Format == "" {
		l.Format = DefaultLogFormat
	}
	l.Lock()
	//	setup our server log template
	l.template = template.New("logfile")

	if _, err := l.template.Parse(l.Format); err != nil {
		log.Printf("Could not parse log template (%v) disabling logging. Error: %v", l.Format, err)
		l.skip = true
	}
	l.Unlock()
}

func (l *Logger) Log(item logItem) {
	l.initTemplate()
	if l == nil || l.File == nil || l.skip {
		return
	}

	if item.Time.IsZero() {
		item.Time = time.Now()
	}

	var lstr string
	lbuf := bytes.NewBufferString(lstr)

	if err := l.template.Execute(lbuf, item); err != nil {
		// Don't care about the error.
		log.Println("Error writing to log file; disabling logging.", err)
		l.Lock()
		l.skip = true
		l.Unlock()
		return
	}
	b := lbuf.Bytes()

	// If there is no new line, let's add it.
	if b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}

	// Don't care about the error.
	l.Lock()
	l.File.Write(b)
	l.Unlock()
}
