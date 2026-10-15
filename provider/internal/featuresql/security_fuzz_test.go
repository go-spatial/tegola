package featuresql

import (
	"errors"
	"reflect"
	"testing"
)

func FuzzFeatureSQLSecurityBoundaries(f *testing.F) {
	for _, s := range []string{"SELECT id,geom FROM items", "SELECT id,geom FROM items WHERE id=1", "SELECT id,geom FROM items; DROP TABLE items", "SELECT id,geom FROM items -- injected", `SELECT "id", "geom" FROM "items" WHERE "id"=1`, "SELECT id,geom FROM items WHERE id=1e1000000000", "SELECT id,geom FROM items WHERE id='\\x'", "\xff"} {
		f.Add(s, uint8(SQLite))
	}
	f.Fuzz(func(t *testing.T, input string, dialect uint8) {
		p, err := Parse(input, Dialect(dialect))
		if err != nil {
			if (!errors.Is(err, ErrInvalid) && !errors.Is(err, ErrUnsupported)) || len(err.Error()) > 512 {
				t.Fatal("untyped or unbounded parse error")
			}
			return
		}
		second, e := Parse(input, Dialect(dialect))
		if e != nil || !reflect.DeepEqual(p, second) {
			t.Fatal("nondeterministic parsing")
		}
	})
}
