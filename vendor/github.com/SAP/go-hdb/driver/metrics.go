package driver

import (
	"slices"
	"sync"
	"time"
)

const (
	counterBytesRead = iota
	counterBytesWritten
	counterSessionConnects
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
	count       uint64
	sum         float64
	upperBounds []float64
	boundCounts []uint64
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

func (h *histogram) add(v float64) {
	h.count++
	if v < 0 { // negative duration (e.g. clock adjustment): clamp into the zero bucket
		v = 0
	}
	h.sum += v
	// determine index
	idx, _ := slices.BinarySearch(h.upperBounds, v)
	for i := idx; i < len(h.upperBounds); i++ {
		h.boundCounts[i]++
	}
}

type msgKind uint8

const (
	msgCounter msgKind = iota
	msgGauge
	msgTime
	msgSQLTime
	msgTimeCounter
)

// metricMsg is the single message type sent to the metrics collector channel.
type metricMsg struct {
	kind msgKind
	idx  int    // metric index: counter, gauge, time, or sqlTime
	idx2 int    // counter index for msgTimeCounter
	v    int64  // gauge value for msgGauge
	c    uint64 // counter value for msgCounter and msgTimeCounter
	d    time.Duration
}

const numMetricCollectorCh = 100

type metrics struct {
	mu     sync.RWMutex
	connMu sync.Mutex // guards n and collector start/stop transitions
	n      int
	wg     *sync.WaitGroup
	msgCh  chan metricMsg

	parentMetrics *metrics

	timeUnit string
	divider  float64

	counters []uint64
	gauges   []int64
	times    []*histogram
	sqlTimes []*histogram
}

func newMetrics(parentMetrics *metrics, timeUnit string, timeUpperBounds []float64) *metrics {
	d, ok := timeUnitMap[timeUnit]
	if !ok {
		panic("invalid unit")
	}
	rv := &metrics{
		wg:            new(sync.WaitGroup),
		parentMetrics: parentMetrics,
		timeUnit:      timeUnit,
		divider:       float64(d),
		counters:      make([]uint64, numCounter),
		gauges:        make([]int64, numGauge),
		times:         make([]*histogram, numTime),
		sqlTimes:      make([]*histogram, numSQLTime),
	}
	for i := range int(numTime) {
		rv.times[i] = newHistogram(timeUpperBounds)
	}
	for i := range int(numSQLTime) {
		rv.sqlTimes[i] = newHistogram(timeUpperBounds)
	}
	return rv
}

func (m *metrics) incrConn() {
	m.connMu.Lock()
	defer m.connMu.Unlock()
	m.n++
	if m.n > 1 {
		return
	}
	m.msgCh = make(chan metricMsg, numMetricCollectorCh)
	m.wg.Go(func() {
		// collect
		for msg := range m.msgCh {
			m.handleMsg(msg)
		}
	})
}

func (m *metrics) decrConn() {
	m.connMu.Lock()
	defer m.connMu.Unlock()
	m.n--
	if m.n > 0 {
		return
	}
	if m.n < 0 {
		panic("unpaired decrConn")
	}
	close(m.msgCh)
	m.wg.Wait()
}

func (m *metrics) stats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

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
		SessionConnects:  m.counters[counterSessionConnects],
		TimeUnit:         m.timeUnit,
		ReadTime:         m.times[timeRead].stats(),
		WriteTime:        m.times[timeWrite].stats(),
		AuthTime:         m.times[timeAuth].stats(),
		SQLTimes:         sqlTimes,
	}
}

func (m *metrics) handleMsg(msg metricMsg) {
	m.mu.Lock()
	switch msg.kind {
	case msgCounter:
		m.counters[msg.idx] += msg.c
	case msgGauge:
		m.gauges[msg.idx] += msg.v
	case msgTime:
		m.times[msg.idx].add(float64(msg.d.Nanoseconds()) / m.divider)
	case msgSQLTime:
		m.sqlTimes[msg.idx].add(float64(msg.d.Nanoseconds()) / m.divider)
	case msgTimeCounter:
		m.times[msg.idx].add(float64(msg.d.Nanoseconds()) / m.divider)
		m.counters[msg.idx2] += msg.c
	default:
		panic("invalid metric message kind")
	}
	m.mu.Unlock()

	if m.parentMetrics != nil {
		m.parentMetrics.handleMsg(msg)
	}
}

// addCounter sends a counter message to the metrics collector channel.
func (m *metrics) addCounter(idx int, v uint64) {
	m.msgCh <- metricMsg{kind: msgCounter, idx: idx, c: v}
}

// addGauge sends a gauge message to the metrics collector channel.
func (m *metrics) addGauge(idx int, v int64) {
	m.msgCh <- metricMsg{kind: msgGauge, idx: idx, v: v}
}

// addTime sends a time message to the metrics collector channel.
func (m *metrics) addTime(idx int, d time.Duration) {
	m.msgCh <- metricMsg{kind: msgTime, idx: idx, d: d}
}

// addTimeValue sends the time elapsed since start to the metrics collector channel.
func (m *metrics) addTimeValue(idx int, start time.Time) {
	m.addTime(idx, time.Since(start))
}

// addSQLTime sends an SQL statement time message to the metrics collector channel.
func (m *metrics) addSQLTime(idx int, d time.Duration) {
	m.msgCh <- metricMsg{kind: msgSQLTime, idx: idx, d: d}
}

// addSQLTimeValue sends the time elapsed since start to the metrics collector channel.
func (m *metrics) addSQLTimeValue(idx int, start time.Time) {
	m.addSQLTime(idx, time.Since(start))
}

// addTimeCounter sends a combined time and counter message to the metrics collector channel.
func (m *metrics) addTimeCounter(idx int, d time.Duration, cidx int, v uint64) {
	m.msgCh <- metricMsg{kind: msgTimeCounter, idx: idx, d: d, idx2: cidx, c: v}
}
