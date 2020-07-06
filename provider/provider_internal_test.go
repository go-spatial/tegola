package provider

import "testing"

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
