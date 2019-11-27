package debugger

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/pborman/uuid"
)

const (
	// ContextRecorderKey the key to store the recorder in the context
	ContextRecorderKey = "debugger_recorder_key"

	// ContextRecorderKey the key to store the testname in the context
	ContextRecorderTestnameKey = "debugger_recorder_testname_key"
)

// DefaultOutputDir is where the system will write the debugging db/files
// By default this will use os.TempDir() to write to the system temp directory
// set this in an init function to wite elsewhere.
var DefaultOutputDir = os.TempDir()

// AsString will create string contains the stringified items seperated by a ':'
func AsString(vs ...interface{}) string {
	var s strings.Builder
	var addc bool
	for _, v := range vs {
		if addc {
			s.WriteString(":")
		}
		fmt.Fprintf(&s, "%v", v)
		addc = true
	}
	return s.String()
}

// GetRecorderFromContext will return the recoder that is
// in the context. If there isn't a recorder, then an invalid
// recorder will be returned. This can be checked with the
// IsValid() function on the recorder.
func GetRecorderFromContext(ctx context.Context) Recorder {

	r, _ := ctx.Value(ContextRecorderKey).(Recorder)

	if name, ok := ctx.Value(ContextRecorderTestnameKey).(string); ok {
		r.Desc.Name = name
	}

	return r
}

var lck sync.Mutex
var recrds = make(map[string]struct {
	fn   string
	rcrd *recorder
})

func cleanupFilename(fn string) string {
	const replaceValues = ` []{}"'^%*&\,;?!()`
	var nfn strings.Builder
	for _, r := range strings.TrimSpace(strings.ToLower(fn)) {
		switch {
		case !unicode.IsPrint(r) || unicode.IsSpace(r):
			// no opt
		case strings.ContainsRune(replaceValues, r):
			nfn.WriteRune('_')
		default:
			nfn.WriteRune(r)
		}
	}
	return nfn.String()
}
func getFilenameDir(initialFilename string) (dir, filename string) {

	initialFilename = cleanupFilename(initialFilename)
	fullFilename := filepath.Clean(filepath.Join(DefaultOutputDir, initialFilename))
	dir = filepath.Dir(fullFilename)
	filename = filepath.Base(fullFilename)
	return dir, filename
}

// AugmentRecorder is will create and configure a new recorder (if needed) to
// be used to track the debugging entries
// A Close call on the recoder should be supplied along with the
// AugmentRecorder call, this is usually done using a defer
// If the testFilename is "", then the function name of the calling function
// will be used as the filename for the database file.
func AugmentRecorder(rec Recorder, testFilename string) (Recorder, bool) {

	if rec.IsValid() {
		rec.IncrementCount()
		return rec, false
	}

	if testFilename == "" {
		testFilename = funcFileLine().Func
	}
	dir, filename := getFilenameDir(testFilename)

	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		panic(fmt.Sprintf("Failed to created dir %v:%v", dir, err))
	}

	lck.Lock()
	defer lck.Unlock()
	rcrd, ok := recrds[testFilename]
	if !ok {
		rcd, filename, err := NewRecorder(dir, filename)
		if err != nil {
			panic(err)
		}
		rcrd = struct {
			fn   string
			rcrd *recorder
		}{
			fn:   filename,
			rcrd: &recorder{Interface: rcd},
		}
		recrds[testFilename] = rcrd
	}
	rcrd.rcrd.IncrementCount()
	if debug {
		log.Println("Writing debugger output to", rcrd.fn)
	}

	return Recorder{
		recorder: rcrd.rcrd,
		Desc: TestDescription{
			Name: uuid.NewRandom().String(),
		},
		Filename: rcrd.fn,
	}, true

}

// AugmentContext is will add and configure the recorder used to track the
// debugging entries into the context.
// A Close call should be supplied along with the AugmentContext  call, this
// is usually done using a defer
// If the testFilename is "", then the function name of the calling function
// will be used as the filename for the database file.
func AugmentContext(ctx context.Context, testFilename string) context.Context {
	if testFilename == "" {
		testFilename = funcFileLine().Func
	}

	if rec, newRec := AugmentRecorder(GetRecorderFromContext(ctx), testFilename); newRec {
		return context.WithValue(ctx, ContextRecorderKey, rec)
	}
	return ctx
}

// Close allows the recorder to release any resources it as, each
// AugmentContext call should have a mirroring Close call that is
// called at the end of the function.
func Close(ctx context.Context) {
	GetRecorderFromContext(ctx).Close()
}

func CloseWait(ctx context.Context) {
	GetRecorderFromContext(ctx).CloseWait()
}
func SetTestName(ctx context.Context, name string) context.Context {
	return context.WithValue(
		ctx,
		ContextRecorderTestnameKey,
		name,
	)
}

// Filename returns the filename of the recorder in the ctx if one exists or an empty string
func Filename(ctx context.Context) string {
	return GetRecorderFromContext(ctx).Filename
}

// Record records the geom and descriptive attributes into the debugging system
func Record(ctx context.Context, geom interface{}, category string, descriptionFormat string, data ...interface{}) {
	RecordFFLOn(GetRecorderFromContext(ctx), FFL(0), geom, category, descriptionFormat, data...)
}

// RecordOn records the geom and descriptive attributes into the debugging system
func RecordOn(rec Recorder, geom interface{}, category string, descriptionFormat string, data ...interface{}) {
	RecordFFLOn(rec, FFL(0), geom, category, descriptionFormat, data...)
}

// RecordFFLOn records the geom and descriptive attributes into the debugging system with the give Func File Line values
func RecordFFLOn(rec Recorder, ffl FuncFileLineType, geom interface{}, category string, descriptionFormat string, data ...interface{}) {
	if !rec.IsValid() {
		return
	}
	description := fmt.Sprintf(descriptionFormat, data...)

	rec.Record(
		geom,
		ffl,
		TestDescription{
			Category:    category,
			Description: description,
		},
	)
}
