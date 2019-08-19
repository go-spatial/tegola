package recorder

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

type TestDescription struct {
	Name        string
	Category    string
	Description string
}

type Interface interface {
	Close() error
	Record(geom interface{}, FFL FuncFileLineType, Description TestDescription) error
}

type FuncFileLineType struct {
	Func       string
	File       string
	LineNumber int
}

func (ffl FuncFileLineType) String() string {
	file := filepath.Base(ffl.File)
	return fmt.Sprintf("%v@%v:%v", file, ffl.LineNumber, ffl.Func)
}

// FuncFileLine returns the func file and line number of the the number of callers
// above the caller of this function. Zero returns the immediate caller above the
// caller of the FuncFileLine func.
func FuncFileLine(lvl uint) FuncFileLineType {
	fnName := "unknown"
	file := "unknown"
	lineNo := -1

	pc, _, _, ok := runtime.Caller(int(lvl) + 2)
	details := runtime.FuncForPC(pc)
	if ok && details != nil {
		fnName = details.Name()
		file, lineNo = details.FileLine(pc)
	}

	vs := strings.Split(fnName, "/")
	fnName = vs[len(vs)-1]

	return FuncFileLineType{
		Func:       fnName,
		File:       file,
		LineNumber: lineNo,
	}
}
