package featuresemantics

import (
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"
)

func TestEpochBoundLiteralRounding(t *testing.T) {
	for _, tc := range []struct {
		name        string
		instant     time.Time
		tail        string
		leap        bool
		floor, ceil [4]string
	}{
		{"zero", time.Unix(0, 0), "", false, [4]string{"0", "0", "0", "0"}, [4]string{"0", "0", "0", "0"}},
		{"negative subunit", time.Unix(-1, 999999999), "", false, [4]string{"-1", "-1", "-1", "-1"}, [4]string{"0", "0", "0", "-1"}},
		{"negative fraction", time.Unix(-2, 500000000), "", false, [4]string{"-2", "-1500", "-1500000", "-1500000000"}, [4]string{"-1", "-1500", "-1500000", "-1500000000"}},
		{"positive subunit", time.Unix(0, 1), "", false, [4]string{"0", "0", "0", "1"}, [4]string{"1", "1", "1", "1"}},
		{"positive exact", time.Unix(2, 500000000), "", false, [4]string{"2", "2500", "2500000", "2500000000"}, [4]string{"3", "2500", "2500000", "2500000000"}},
		{"zero tail", time.Unix(0, 0), "0", false, [4]string{"0", "0", "0", "0"}, [4]string{"0", "0", "0", "0"}},
		{"multiple zero tail", time.Unix(0, 0), "000", false, [4]string{"0", "0", "0", "0"}, [4]string{"0", "0", "0", "0"}},
		{"nonzero tail", time.Unix(0, 0), "1", false, [4]string{"0", "0", "0", "0"}, [4]string{"1", "1", "1", "1"}},
		{"leading zero tail", time.Unix(0, 0), "01", false, [4]string{"0", "0", "0", "0"}, [4]string{"1", "1", "1", "1"}},
		{"negative tail", time.Unix(-1, 999999999), "01", false, [4]string{"-1", "-1", "-1", "-1"}, [4]string{"0", "0", "0", "0"}},
		{"leap", time.Unix(1483228799, 0), "", true, [4]string{"1483228799", "1483228799999", "1483228799999999", "1483228799999999999"}, [4]string{"1483228800", "1483228800000", "1483228800000000", "1483228800000000000"}},
		{"leap fraction", time.Unix(1483228799, 900000000), "01", true, [4]string{"1483228799", "1483228799999", "1483228799999999", "1483228799999999999"}, [4]string{"1483228800", "1483228800000", "1483228800000000", "1483228800000000000"}},
		{"far future", time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), "", false, [4]string{"253370764800", "253370764800000", "253370764800000000", "253370764800000000000"}, [4]string{"253370764800", "253370764800000", "253370764800000000", "253370764800000000000"}},
		{"far past", time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), "", false, [4]string{"-62135596800", "-62135596800000", "-62135596800000000", "-62135596800000000000"}, [4]string{"-62135596800", "-62135596800000", "-62135596800000000", "-62135596800000000000"}},
	} {
		for i, scale := range []int64{1, 1000, 1000000, 1000000000} {
			t.Run(fmt.Sprintf("%s/%d", tc.name, scale), func(t *testing.T) {
				for _, ceil := range []bool{false, true} {
					want := tc.floor[i]
					if ceil {
						want = tc.ceil[i]
					}
					if got := EpochBound(tc.instant, scale, ceil, tc.tail, tc.leap); got.String() != want {
						t.Fatalf("ceil=%v got %s want %s", ceil, got, want)
					}
				}
			})
		}
	}
}

func TestEpochBoundSourceInt64Extremes(t *testing.T) {
	for _, tc := range []struct {
		instant time.Time
		tail    string
		ceil    bool
		want    string
		fits    bool
	}{
		{time.Unix(9223372036, 854775807), "", false, "9223372036854775807", true},
		{time.Unix(9223372036, 854775807), "1", true, "9223372036854775808", false},
		{time.Unix(-9223372037, 145224192), "", false, "-9223372036854775808", true},
		{time.Unix(-9223372037, 145224191), "", false, "-9223372036854775809", false},
	} {
		got := EpochBound(tc.instant, 1000000000, tc.ceil, tc.tail, false)
		if got.String() != tc.want || got.IsInt64() != tc.fits {
			t.Fatalf("got %s fits=%v want %s fits=%v", got, got.IsInt64(), tc.want, tc.fits)
		}
	}
}

func TestEpochBoundDetachedConcurrent(t *testing.T) {
	instant := time.Unix(-1, 999999999)
	first := EpochBound(instant, 1000000000, true, "01", false)
	first.SetInt64(123)
	if got := EpochBound(instant, 1000000000, true, "01", false); got.Sign() != 0 {
		t.Fatalf("retained result %s", got)
	}
	var workers sync.WaitGroup
	for i := 0; i < 32; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 100; j++ {
				got := EpochBound(instant, 1000000000, true, "01", false)
				if got.Cmp(big.NewInt(0)) != 0 {
					t.Errorf("concurrent result %s", got)
				}
				got.SetInt64(999)
			}
		}()
	}
	workers.Wait()
}
