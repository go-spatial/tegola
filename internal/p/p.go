// pacakge p takes in values and returns a pointer to the value
package p

import "time"

func Bool(b bool) *bool {
	return &b
}

func String(str string) *string {
	return &str
}

func Int(i int) *int {
	return &i
}

func Int32(i int32) *int32 {
	return &i
}

func Int64(i int64) *int64 {
	return &i
}

func Uint(i uint) *uint {
	return &i
}

func Uint32(i uint32) *uint32 {
	return &i
}

func Uint64(i uint64) *uint64 {
	return &i
}

func Float32(f float32) *float32 {
	return &f
}

func Float64(f float64) *float64 {
	return &f
}

func Time(t time.Time) *time.Time {
	return &t
}
