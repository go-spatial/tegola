package provider

import (
	"time"

	"github.com/go-spatial/geom"
)

const (
	TimeFiltererType     = "time"
	ExtentFiltererType   = "extent"
	IndexFiltererType    = "index"
	PropertyFiltererType = "property"
)

type BaseFilterer interface {
	// The type of filter this is
	Type() string
}

// --- Time
type TimeFilterer interface {
	BaseFilterer
	// use the isZero() method on time to know if a time is set
	Start() time.Time
	End() time.Time
}

type TimePeriod struct {
	start time.Time
	end   time.Time
}

func (tp TimePeriod) Init(itp [2]time.Time) TimePeriod {
	tp.start = itp[0]
	tp.end = itp[1]
	return tp
}

func (tp *TimePeriod) Start() time.Time {
	return tp.start
}

func (tp *TimePeriod) End() time.Time {
	return tp.end
}

func (_ *TimePeriod) Type() string { return TimeFiltererType }

// --- Extent (BBox)
type ExtentFilterer interface {
	BaseFilterer
	geom.Extenter
}

type ExtentFilter struct {
	ext *geom.Extent
}

func (e ExtentFilter) Init(ext geom.Extent) ExtentFilter {
	e.ext = &ext
	return e
}

func (e *ExtentFilter) Extent() geom.Extent { return *e.ext }

func (_ *ExtentFilter) Type() string { return ExtentFiltererType }

// --- Index
type IndexFilterer interface {
	BaseFilterer
	Start() uint
	End() uint
}

type IndexRange struct {
	startIdx uint
	endIdx   uint
}

func (i IndexRange) Init(indices [2]uint) IndexRange {
	i.startIdx = indices[0]
	i.endIdx = indices[1]
	return i
}

func (i *IndexRange) Start() uint {
	return i.startIdx
}

func (i *IndexRange) End() uint {
	return i.endIdx
}

func (_ *IndexRange) Type() string { return ExtentFiltererType }

// --- Property
// Properties to filter features on.
// If the feature has any of the properties named, the property values must match (fuzzily, i.e. conversion
//	from string to native type then match) for the feature to be returned.
type PropertyFilterer interface {
	BaseFilterer
	Map() map[string]interface{}
}

type Properties map[string]interface{}

func (p Properties) Init(ip map[string]interface{}) Properties {
	for k, v := range ip {
		p[k] = v
	}
	return p
}

func (_ Properties) Type() string { return PropertyFiltererType }

func (p Properties) Map() map[string]interface{} {
	return p
}
