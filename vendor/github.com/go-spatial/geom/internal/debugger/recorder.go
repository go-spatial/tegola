package debugger

import (
	"log"
	"sync"

	recdr "github.com/go-spatial/geom/internal/debugger/recorder"
)

type TestDescription = recdr.TestDescription

type recorder struct {
	recdr.Interface

	wg sync.WaitGroup

	clck sync.Mutex
	// Number of times the DB connection has been "initilized", and expect
	// the same number of close statements, but only want to close on the
	// last close() statement.
	count  uint
	closed bool
}

// IncrementCount used for reference counting for when to release
// resources; each thing holding a copy of this resource should
// call this if it intends to call Close()
func (rec *recorder) IncrementCount() {
	if rec == nil {
		return
	}
	rec.clck.Lock()
	rec.count++
	rec.clck.Unlock()
}

// Close will allows the recorder to free up any held resources
func (rec *recorder) Close() error {
	if rec == nil {
		return nil
	}
	rec.clck.Lock()
	rec.count--
	c := rec.count
	if !rec.closed && c <= 0 {
		rec.closed = true
		rec.clck.Unlock()
		log.Println("Waiting for things to finish")
		rec.wg.Wait()
		log.Println("Done waiting for things to finish")
		return rec.Interface.Close()
	}
	rec.clck.Unlock()
	return nil
}

// Close will allows the recorder to free up any held resources
func (rec *recorder) CloseWait() error {
	if rec == nil {
		return nil
	}
	rec.clck.Lock()
	rec.count--
	c := rec.count
	if !rec.closed && c <= 0 {
		rec.closed = true
		rec.clck.Unlock()
		if debug {
			log.Println("waiting for things to finish")
		}
		rec.wg.Wait()
		if debug {
			log.Println("done waiting for things to finish")
		}
		return rec.Interface.Close()
	}
	rec.clck.Unlock()
	if debug {
		log.Println("waiting for things to finish")
	}
	rec.wg.Wait()
	return nil
}

// Closed will report if the database is available for writing
func (rec *recorder) Closed() bool {
	if rec == nil {
		return true
	}
	rec.clck.Lock()
	defer rec.clck.Unlock()
	return rec.closed
}

// Recorder is used to record entries into the debugging database
type Recorder struct {
	*recorder

	// Desc is the template for the description to use when recording a
	// test.
	Desc TestDescription

	// The filename the recording are being written to, or empty
	Filename string
}

// IsValid is the given Recorder valid
func (rec Recorder) IsValid() bool { return !rec.recorder.Closed() }

// Record will record an entry into the debugging Database. Zero values in the desc will be
// replaced by their corrosponding values in the Recorder.Desc
func (rec Recorder) Record(geom interface{}, ffl FuncFileLineType, desc TestDescription) error {
	if !rec.IsValid() {
		return nil
	}
	tstDesc := rec.Desc
	if desc.Name != "" {
		tstDesc.Name = desc.Name
	}
	if desc.Category != "" {
		tstDesc.Category = desc.Category
	}
	if desc.Description != "" {
		tstDesc.Description = desc.Description
	}
	return rec.recorder.Record(geom, ffl, tstDesc)
}

// AsyncRecord will record an entry into the debugging Database asynchronously. Zero values in the desc will be
// replaced by their corrosponding values in the Recorder.Desc
func (rec Recorder) AsyncRecord(geom interface{}, ffl FuncFileLineType, desc TestDescription) {
	if !rec.IsValid() {
		return
	}
	tstDesc := rec.Desc
	if desc.Name != "" {
		tstDesc.Name = desc.Name
	}
	if desc.Category != "" {
		tstDesc.Category = desc.Category
	}
	if desc.Description != "" {
		tstDesc.Description = desc.Description
	}
	rec.recorder.wg.Add(1)
	go func() {
		err := rec.recorder.Record(geom, ffl, tstDesc)
		rec.recorder.wg.Done()
		if err != nil {
			log.Println("Got an error running recorder (async):", err)
		}
	}()
}
