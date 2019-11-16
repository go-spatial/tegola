package cmd

import (
	"strconv"
	"strings"
)

func IsValidLat(f64 float64) bool { return -90.0 <= f64 && f64 <= 90.0 }

func IsValidLatString(lat string) (float64, bool) {
	f64, err := strconv.ParseFloat(strings.TrimSpace(lat), 64)
	return f64, err == nil || IsValidLat(f64)
}

func IsValidLng(f64 float64) bool { return -180.0 <= f64 && f64 <= 180.0 }

func IsValidLngString(lng string) (float64, bool) {
	f64, err := strconv.ParseFloat(strings.TrimSpace(lng), 64)
	return f64, err == nil || IsValidLng(f64)
}
