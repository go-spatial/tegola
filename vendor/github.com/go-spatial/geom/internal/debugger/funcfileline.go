package debugger

import (
	recdr "github.com/go-spatial/geom/internal/debugger/recorder"
)

type FuncFileLineType = recdr.FuncFileLineType

// FuncFileLine returns the func file and line number of the the number of callers
// above the caller of this function. Zero returns the immediate caller above the
// caller of the FuncFileLine func.
func FuncFileLine(lvl uint) (string, string, int) {
	ffl := recdr.FuncFileLine(lvl + 1)
	return ffl.Func, ffl.File, ffl.LineNumber
}

func FFL(lvl uint) FuncFileLineType { return recdr.FuncFileLine(lvl + 1) }

func funcFileLine() FuncFileLineType { return recdr.FuncFileLine(1) }
