package testhelpers

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/terranodo/tegola/maths"
)

var validXY *regexp.Regexp

func init() {
	validXY = regexp.MustCompile(`[Xx]\s*:\s*-?\d+(\.\d*)?|[Yy]\s*:\s*-?\d+(\.\d*)?`)
}

func LoadLines(r io.Reader) (lines []maths.Line) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		txt := scanner.Text()
		if strings.Index(txt, "#") == 0 {
			// This is a comment. skip
			continue
		}
		vals := validXY.FindAllStringIndex(txt, -1)
		if len(vals) < 4 {
			// skip lines with less then four points.
			continue
		}
		stridx := validXY.FindAllStringIndex(txt, -1)
		lookForX := true
		isFirstPoint := true
		var pt1, pt2 maths.Pt
		for i := range stridx {
			str := txt[stridx[i][0]:stridx[i][1]]
			k := strings.Index(str, ":")
			if k == -1 {
				// should never happen.
				panic(fmt.Sprintln("Did not find `:` in [", str, "] part of:", txt))
				continue
			}

			n, err := strconv.ParseFloat(strings.TrimSpace(str[k+1:]), 64)
			if err != nil {
				fmt.Println("error converting :", txt)
				panic(err)
			}
			if str[0] == 'X' || str[0] == 'x' {
				if !lookForX {
					panic(fmt.Sprintln("Was expecting to find Y but got another X.", txt))
				}
				lookForX = false
				if isFirstPoint {
					pt1.X = n
					continue
				}
				pt2.X = n
				continue
			}
			if str[0] == 'Y' || str[0] == 'y' {
				if lookForX {
					panic(fmt.Sprintln("Was expecting to find X but got another Y.", txt))
				}
				lookForX = true
				if isFirstPoint {
					pt1.Y = n
					isFirstPoint = false
					continue
				}
				pt2.Y = n
				lines = append(lines, maths.Line{
					maths.Pt{X: pt1.X, Y: pt1.Y},
					maths.Pt{X: pt2.X, Y: pt2.Y},
				})
				isFirstPoint = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	return lines
}
func LoadLinesFromFile(filename string) []maths.Line {
	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	return LoadLines(file)

}

func TestLoadLines(t *testing.T) {
	lines := LoadLinesFromFile("testdata/test1.txt")
	fmt.Println("lines:", lines)

}
