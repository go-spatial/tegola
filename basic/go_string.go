package basic

import "fmt"

func (l Line) GoString() string {
	str := fmt.Sprintf("\n[%v--%v]{\n\t", len(l), l.Direction())
	count := 0
	for i, p := range l {
		if i != 0 {
			str += ","
		}
		str += fmt.Sprintf("(%v,%v)", p[0], p[1])
		if count == 10 {
			str += "\n\t"
			count = 0
		}
		count++
	}
	str += "\n}"
	return str
}
