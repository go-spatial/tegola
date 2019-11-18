/*
Package debugger provides a way for us to capture partial
geometries during geometry processing. The geometries are
stored in a `spatailite` database.

 The general way to use the package is with a `context.Context`
 variable. The package uses context as a way to pass around
 the recorders, that can easily be disabled.

 An example of how this would be used in a function doing work:


	func Foo(ctx context.Context, ... ) (...) {
		// At the top of the package we usually
		// will enhance the context
		if debug {
			ctx = debugger.AugmentContext(ctx, "")
			defer debugger.Close(ctx)
		}

		... // Do things till you want to record the state for

		if debug {
			for i, seg := ranage segments {
				debugger.Record(ctx,
					seg,
					"A Category",
					"Description Format %v", i,
				)
			}
		}

		... // more work

	}


To use the package in a test, one would do something similar.
In your test function, call the `debugger.SetTestName`, with
the test name. The best way to do this is via the `t.Name()`
function in a `testing.T` object. The `context.Context` variable
will need to be Augmented as well -- the first Augment call
in a chain takes precedent, so in each of your functions you
can call this with no worries.

See the following example:


	func TestFoo(t *testing.T){
		type tcase struct {
			...
		}

		fn := func(ctx context.Context, tc tcase) func(*testing.T){
			return func(t *testing.T){

				if debug {
					debugger.SetTestName(ctx, t.Name())
				}

				...
				... = Foo(...)
				...

				if got != tc.expected {
				// record the inputs
					if debug {
						debugger.Record(ctx,
							got,
							debugger.CategoryGot,
							"got segments",
						)
						debugger.Record(ctx,
							tc.expected,
							debugger.CategoryExpected,
							"expected segments",
						)
						debugger.Record(ctx,
							tc.input,
							debugger.CategoryInput,
							"input polygon",
						)
					}
				}
			}
		}

		tests := [...]tcase{ ... }

		ctx := context.Background()

		if debug {
			ctx = debugger.AugmentContext(ctx, "")
			defer debugger.Close(ctx)
		}

		for _, tc := range tests {
			t.Run(tc.name, fn(ctx, tc))
		}
	}


*/
package debugger
