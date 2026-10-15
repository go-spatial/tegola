package provider

import (
	"context"
	"errors"
	"math"
	"sort"
	"testing"

	"github.com/alexeydott/tegola/dict"
)

// TestConvertFeatureIDInvalidValues (audit P6-10) pins the feature-ID
// conversion contract: values that do not fit a uint64 ID — negative,
// fractional or out-of-range numbers — are rejected with an error instead
// of being silently wrapped (-1 used to become 2^64-1) or truncated
// (1.5 used to become 1).
func TestConvertFeatureIDInvalidValues(t *testing.T) {
	tcs := []struct {
		name string
		val  interface{}
	}{
		{"negative float", float64(-1)},
		{"fractional float", float64(1.5)},
		{"NaN float", math.NaN()},
		{"+Inf float", math.Inf(1)},
		{"float past uint64 range", math.Pow(2, 64)},
		{"negative int64", int64(-1)},
		{"negative int32", int32(-5)},
		{"negative int8", int8(-1)},
		{"negative numeric string", "-1"},
		{"fractional numeric string", "1.5"},
		{"non-numeric string", "abc"},
		{"negative numeric bytes", []byte("-1")},
		{"nil", nil},
		{"unsupported type", struct{}{}},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ConvertFeatureID(tc.val)
			if err == nil {
				t.Fatalf("ConvertFeatureID(%v (%T)) = %d, want a rejection error", tc.val, tc.val, got)
			}
			if got != 0 {
				t.Fatalf("ConvertFeatureID(%v) = %d alongside error, want 0", tc.val, got)
			}
		})
	}
}

// TestConvertFeatureIDValidValues pins the values ConvertFeatureID must keep
// accepting after the P6-10 hardening.
func TestConvertFeatureIDValidValues(t *testing.T) {
	tcs := []struct {
		name string
		val  interface{}
		want uint64
	}{
		{"float64 integer", float64(42), 42},
		{"float64 large integer", float64(1e18), 1000000000000000000},
		{"int64", int64(7), 7},
		{"int64 zero", int64(0), 0},
		{"uint64 max", uint64(18446744073709551615), 18446744073709551615},
		{"uint", uint(3), 3},
		{"int8", int8(9), 9},
		{"uint8", uint8(10), 10},
		{"int32", int32(11), 11},
		{"uint32", uint32(13), 13},
		{"uint16", uint16(15), 15},
		{"numeric string", "123", 123},
		{"numeric bytes", []byte("456"), 456},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ConvertFeatureID(tc.val)
			if err != nil {
				t.Fatalf("ConvertFeatureID(%v (%T)) unexpected error: %v", tc.val, tc.val, err)
			}
			if got != tc.want {
				t.Fatalf("ConvertFeatureID(%v (%T)) = %d, want %d", tc.val, tc.val, got, tc.want)
			}
		})
	}
}

func TestProviderFilterInclude(t *testing.T) {

	type tcase struct {
		Expected providerFilter
		Filters  []providerType
		IsMVT    bool
		IsSTD    bool
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {

			got := providerFilterInclude(tc.Filters...)
			if got != tc.Expected {
				t.Errorf("providerFilterInclude, expected %v got %v", tc.Expected, got)
				return
			}
			is := got.Is(TypeStd)
			if is != tc.IsSTD {
				t.Errorf("IsStd, expected %v got %v", tc.IsSTD, is)
			}
			is = got.Is(TypeMvt)
			if is != tc.IsMVT {
				t.Errorf("IsMVT, expected %v got %v", tc.IsMVT, is)
			}
		}
	}
	tests := map[string]tcase{
		"none":  {},
		"none2": {Filters: []providerType{}},
		"std": {
			Expected: 0b00000001,
			Filters:  []providerType{TypeStd},
			IsSTD:    true,
		},
		"mvt": {
			Expected: 0b00000010,
			Filters:  []providerType{TypeMvt},
			IsMVT:    true,
		},
		"all": {
			Expected: 0b00000011,
			Filters:  []providerType{TypeStd, TypeMvt},
			IsSTD:    true,
			IsMVT:    true,
		},
		"all2": {
			Expected: 0b00000011,
			Filters:  []providerType{TypeMvt, TypeStd},
			IsSTD:    true,
			IsMVT:    true,
		},
	}
	for name, tc := range tests {
		t.Run(name, fn(tc))
	}

}

// TestTypeAllCombinesProviderTypes (audit P6-23) guards against TypeAll being
// computed as the intersection (&) of TypeStd and TypeMvt, which evaluates to
// 0 and silently broke the Drivers filter for "all types" requests.
func TestTypeAllCombinesProviderTypes(t *testing.T) {
	if TypeAll != TypeStd|TypeMvt {
		t.Errorf("TypeAll: expected %08b (%d), got %08b (%d)", TypeStd|TypeMvt, TypeStd|TypeMvt, TypeAll, TypeAll)
	}
	if uint8(TypeAll) == 0 {
		t.Error("TypeAll is 0: Drivers() can never filter on all provider types")
	}
}

// TestDriversFilterAllTypeCombos (audit P6-23) exercises the Drivers() type
// filter with every combination of TypeStd/TypeMvt/TypeAll. The pre-fix
// TypeAll = TypeStd & TypeMvt == 0 made Drivers(TypeStd, TypeMvt) return only
// MVT drivers instead of all drivers.
func TestDriversFilterAllTypeCombos(t *testing.T) {
	savedProviders := providers
	defer func() { providers = savedProviders }()

	var fakeStd InitFunc = func(dict.Dicter, []Map) (Tiler, error) { return nil, nil }
	var fakeMvt MVTInitFunc = func(dict.Dicter, []Map) (MVTTiler, error) { return nil, nil }

	providers = map[string]pfns{
		"p-std":  {init: fakeStd},
		"p-mvt":  {mvtInit: fakeMvt},
		"p-both": {init: fakeStd, mvtInit: fakeMvt},
	}

	tests := map[string]struct {
		Filters  []providerType
		Expected []string
	}{
		"no filter":      {nil, []string{"p-both", "p-mvt", "p-std"}},
		"empty filter":   {[]providerType{}, []string{"p-both", "p-mvt", "p-std"}},
		"std only":       {[]providerType{TypeStd}, []string{"p-both", "p-std"}},
		"mvt only":       {[]providerType{TypeMvt}, []string{"p-both", "p-mvt"}},
		"std and mvt":    {[]providerType{TypeStd, TypeMvt}, []string{"p-both", "p-mvt", "p-std"}},
		"mvt and std":    {[]providerType{TypeMvt, TypeStd}, []string{"p-both", "p-mvt", "p-std"}},
		"explicit all":   {[]providerType{TypeAll}, []string{"p-both", "p-mvt", "p-std"}},
		"all with std":   {[]providerType{TypeAll, TypeStd}, []string{"p-both", "p-mvt", "p-std"}},
		"all duplicates": {[]providerType{TypeAll, TypeAll}, []string{"p-both", "p-mvt", "p-std"}},
		"std duplicates": {[]providerType{TypeStd, TypeStd}, []string{"p-both", "p-std"}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := Drivers(tc.Filters...)
			sort.Strings(got)
			if len(got) != len(tc.Expected) {
				t.Fatalf("Drivers(%v): expected %v, got %v", tc.Filters, tc.Expected, got)
			}
			for i := range got {
				if got[i] != tc.Expected[i] {
					t.Fatalf("Drivers(%v): expected %v, got %v", tc.Filters, tc.Expected, got)
				}
			}
		})
	}
}

// fakes for TestIsTileJSONV3Compatible asserting the LayerFielder contract.

type fakeStdFielder struct{}

func (fakeStdFielder) Layers() ([]LayerInfo, error) { return nil, nil }
func (fakeStdFielder) TileFeatures(context.Context, string, Tile, Params, func(*Feature) error) error {
	return nil
}
func (fakeStdFielder) LayerFields(context.Context, string) (map[string]any, error) { return nil, nil }

type fakeStdPlain struct{}

func (fakeStdPlain) Layers() ([]LayerInfo, error) { return nil, nil }
func (fakeStdPlain) TileFeatures(context.Context, string, Tile, Params, func(*Feature) error) error {
	return nil
}

type fakeMVTFielder struct{}

func (fakeMVTFielder) Layers() ([]LayerInfo, error) { return nil, nil }
func (fakeMVTFielder) MVTForLayers(context.Context, Tile, Params, []Layer) ([]byte, error) {
	return nil, nil
}
func (fakeMVTFielder) LayerFields(context.Context, string) (map[string]any, error) { return nil, nil }

type fakeMVTPlain struct{}

func (fakeMVTPlain) Layers() ([]LayerInfo, error) { return nil, nil }
func (fakeMVTPlain) MVTForLayers(context.Context, Tile, Params, []Layer) ([]byte, error) {
	return nil, nil
}

// TestIsTileJSONV3Compatible (audit P6-5) guards the MVT branch of
// TilerUnion.IsTileJSONV3Compatible: it asserted tu.Std.(LayerFielder) while
// Std is nil for MVT providers, so every MVT provider was rejected
// regardless of whether it implemented LayerFielder.
func TestIsTileJSONV3Compatible(t *testing.T) {
	tests := map[string]struct {
		TU      TilerUnion
		WantOK  bool
		WantErr error
	}{
		"mvt with LayerFielder": {
			TU:     TilerUnion{Mvt: fakeMVTFielder{}},
			WantOK: true,
		},
		"mvt without LayerFielder": {
			TU:      TilerUnion{Mvt: fakeMVTPlain{}},
			WantOK:  false,
			WantErr: ErrNotTileJSONV3Compatible,
		},
		"std with LayerFielder": {
			TU:     TilerUnion{Std: fakeStdFielder{}},
			WantOK: true,
		},
		"std without LayerFielder": {
			TU:      TilerUnion{Std: fakeStdPlain{}},
			WantOK:  false,
			WantErr: ErrNotTileJSONV3Compatible,
		},
		"no provider": {
			TU:      TilerUnion{},
			WantOK:  false,
			WantErr: ErrNoProvider,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tc.TU.IsTileJSONV3Compatible()
			if tc.WantErr != nil {
				if !errors.Is(err, tc.WantErr) {
					t.Fatalf("expected error %v, got %v", tc.WantErr, err)
				}
			} else if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if got != tc.WantOK {
				t.Fatalf("expected compatible=%v, got %v", tc.WantOK, got)
			}
		})
	}
}
