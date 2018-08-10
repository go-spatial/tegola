package assert

type Equality struct {
	// Message is the prefix message for the assersion
	Message string
	// Expected is the expected value for the assersion
	Expected string
	// Got is the actual value
	Got string
	// Continue used to determine weather or not to continue with further assertions.
	IsEqual bool
}

func (e Equality) Error() string {
	return e.Message + ", Expected " + e.Expected + " Got " + e.Got
}
func (e Equality) String() string { return e.Error() }

func ErrorEquality(expErr, gotErr error) Equality {
	if expErr != gotErr {
		// could be because test.err == nil and err != nil.
		if expErr == nil && gotErr != nil {
			return Equality{
				"unexpected error",
				"nil",
				gotErr.Error(),
				false,
			}
		}
		if expErr != nil && gotErr == nil {
			return Equality{
				"expected error",
				expErr.Error(),
				"nil",
				false,
			}
		}
		if expErr.Error() != gotErr.Error() {
			return Equality{
				"incorrect error value",
				expErr.Error(),
				gotErr.Error(),
				false,
			}
		}
		return Equality{IsEqual: true}
	}
	if expErr != nil {
		// No need to look at other values, expected an error.
		return Equality{}
	}
	return Equality{IsEqual: true}
}
