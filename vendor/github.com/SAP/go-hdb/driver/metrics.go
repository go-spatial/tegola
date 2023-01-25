package driver

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/exp/slices"
)

const (
	counterBytesRead = iota
	counterBytesWritten
	numCounter
)

const (
	gaugeConn = iota
	gaugeTx
	gaugeStmt
	numGauge
)

const (
	timeRead = iota
	timeWrite
	timeAuth
	numTime
)

const (
	sqlTimeQuery = iota
	sqlTimePrepare
	sqlTimeExec
	sqlTimeCall
	sqlTimeFetch
	sqlTimeFetchLob
	sqlTimeRollback
	sqlTimeCommit
	numSQLTime
)

type histogram struct {
	count          uint64
	sum            float64
	upperBounds    []float64
	boundCounts    []uint64
	underflowCount uint64 // in case of negative duration (will add to zero bucket)
}

func newHistogram(upperBounds []float64) *histogram {
	return &histogram{upperBounds: upperBounds, boundCounts: make([]uint64, len(upperBounds))}
}

func (h *histogram) stats() *StatsHistogram {
	rv := &StatsHistogram{
		Count:   h.count,
		Sum:     h.sum,
		Buckets: make(map[float64]uint64, len(h.upperBounds)),
	}
	for i, upperBound := range h.upperBounds {
		rv.Buckets[upperBound] = h.boundCounts[i]
	}
	return rv
}

func (h *histogram) add(v float64) { // time in nanoseconds
	h.count++
	if v < 0 {
		h.underflowCount++
		v = 0
	}
	h.sum += v
	// determine index
	idx, _ := slices.BinarySearch(h.upperBounds, v)
	for i := idx; i < len(h.upperBounds); i++ {
		h.boundCounts[i]++
	}
}

type counterMsg struct {
	v   uint64
	idx int
}

type gaugeMsg struct {
	v   int64
	idx int
}

type timeMsg struct {
	d   time.Duration
	idx int
}

type sqlTimeMsg struct {
	d   time.Duration
	idx int
}

type metrics struct {
	parent *metrics

	counters []uint64
	gauges   []int64
	times    []*histogram
	sqlTimes []*histogram

	wg    *sync.WaitGroup
	chMsg chan any

	closed atomicBool
}

const (
	numCh = 100000
)

func newMetrics(parent *metrics, timeUpperBounds []float64) *metrics {
	rv := &metrics{
		parent:   parent,
		counters: make([]uint64, numCounter),
		gauges:   make([]int64, numGauge),
		times:    make([]*histogram, numTime),
		sqlTimes: make([]*histogram, numSQLTime),

		wg:    new(sync.WaitGroup),
		chMsg: make(chan any, numCh),
	}
	for i := 0; i < int(numTime); i++ {
		rv.times[i] = newHistogram(timeUpperBounds)
	}
	for i := 0; i < int(numSQLTime); i++ {
		rv.sqlTimes[i] = newHistogram(timeUpperBounds)
	}
	rv.wg.Add(1)
	if parent == nil {
		go rv.collect(rv.wg, rv.chMsg, rv.handleMsg)
	} else {
		go rv.collect(rv.wg, rv.chMsg, rv.handleParentMsg)
	}
	return rv
}

func (m *metrics) buildStats() *Stats {
	sqlTimes := make(map[string]*StatsHistogram, len(m.sqlTimes))
	for i, sqlTime := range m.sqlTimes {
		sqlTimes[statsCfg.SQLTimeTexts[i]] = sqlTime.stats()
	}
	return &Stats{
		OpenConnections:  int(m.gauges[gaugeConn]),
		OpenTransactions: int(m.gauges[gaugeTx]),
		OpenStatements:   int(m.gauges[gaugeStmt]),
		ReadBytes:        m.counters[counterBytesRead],
		WrittenBytes:     m.counters[counterBytesWritten],
		ReadTime:         m.times[timeRead].stats(),
		WriteTime:        m.times[timeWrite].stats(),
		AuthTime:         m.times[timeAuth].stats(),
		SQLTimes:         sqlTimes,
	}
}

func milliseconds(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }

func (m *metrics) handleMsg(msg any) {
	switch msg := msg.(type) {
	case counterMsg:
		m.counters[msg.idx] += msg.v
	case gaugeMsg:
		m.gauges[msg.idx] += msg.v
	case timeMsg:
		m.times[msg.idx].add(milliseconds(msg.d))
	case sqlTimeMsg:
		m.sqlTimes[msg.idx].add(milliseconds(msg.d))
	case chan *Stats:
		msg <- m.buildStats()
	default:
		panic(fmt.Sprintf("invalid metric message type %T", msg))
	}
}

func (m *metrics) handleParentMsg(msg any) {
	switch msg := msg.(type) {
	case counterMsg:
		m.parent.chMsg <- msg
		m.counters[msg.idx] += msg.v
	case gaugeMsg:
		m.parent.chMsg <- msg
		m.gauges[msg.idx] += msg.v
	case timeMsg:
		m.parent.chMsg <- msg
		m.times[msg.idx].add(milliseconds(msg.d))
	case sqlTimeMsg:
		m.parent.chMsg <- msg
		m.sqlTimes[msg.idx].add(milliseconds(msg.d))
	case chan *Stats:
		msg <- m.buildStats()
	default:
		panic(fmt.Sprintf("invalid metric message type %T", msg))
	}
}

func (m *metrics) collect(wg *sync.WaitGroup, chMsg <-chan any, msgHandler func(msg any)) {
	for msg := range chMsg {
		msgHandler(msg)
	}
	wg.Done()
}

func (m *metrics) stats() *Stats {
	if m.closed.Load() { // if closed return stas directly as we do not have write conflicts anymore
		return m.stats()
	}
	chStats := make(chan *Stats)
	m.chMsg <- chStats
	return <-chStats
}

func (m *metrics) close() {
	m.closed.Store(true)
	close(m.chMsg)
	m.wg.Wait()
}
