package prometheus

import (
	"strconv"
	"time"

	tegolaCache "github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/observability"
	"github.com/prometheus/client_golang/prometheus"
)

type cache struct {
	observeVars       []string
	cache             tegolaCache.Interface
	hitsCounter       *prometheus.CounterVec
	missesCounter     *prometheus.CounterVec
	inFlightGauge     prometheus.Gauge
	durationSeconds   *prometheus.HistogramVec
	responseSizeBytes *prometheus.HistogramVec
	errors            *prometheus.CounterVec
}

func newCache(registry prometheus.Registerer, prefix string, observeVars []string, subCache tegolaCache.Interface) *cache {
	var c = cache{
		observeVars: observeVars,
		cache:       subCache,
	}

	c.inFlightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: prefix + "_flight_requests",
		Help: "A gauge of requests currently being handled by the cache",
	})
	names := c.labelNames()

	c.hitsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prefix + "_hits_total",
			Help: "A counter of the number of tile hits",
		},
		names,
	)
	c.missesCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prefix + "_misses_total",
			Help: "A counter of the number of tile misses",
		},
		names,
	)

	c.durationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    prefix + "_duration_seconds",
			Help:    "A histogram of latencies for requests.",
			Buckets: []float64{.25, .5, 1, 2.5, 5, 10},
		},
		names,
	)

	c.responseSizeBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    prefix + "_response_size_bytes",
			Help:    "A histogram of response sizes for requests.",
			Buckets: []float64{float64(500 * kb), float64(1 * mb), float64(5 * mb)},
		},
		names,
	)

	// Register our variables
	registry.MustRegister(
		c.inFlightGauge,
		c.hitsCounter,
		c.missesCounter,
		c.durationSeconds,
		c.responseSizeBytes,
	)

	return &c
}

// labelNames returns the label name based on the configured observeVars and "sub_command"
func (co *cache) labelNames() (names []string) {
	names = []string{"sub_command"}
	for _, keyName := range co.observeVars {
		switch keyName {
		case observability.ObserveVarMapName:
			names = append(names, "map_name")
		case observability.ObserveVarLayerName:
			names = append(names, "layer_name")
		case observability.ObserveVarTileX:
			names = append(names, "x")
		case observability.ObserveVarTileY:
			names = append(names, "y")
		case observability.ObserveVarTileZ:
			names = append(names, "z")
		}
	}
	return names
}

// labels returns prometheus.Labels based on the configured observeVars
func (co *cache) labels(cmd string, key *tegolaCache.Key) (lbs prometheus.Labels) {
	lbs = make(prometheus.Labels)
	for _, keyName := range co.observeVars {
		switch keyName {
		case observability.ObserveVarMapName:
			lbs["map_name"] = key.MapName
		case observability.ObserveVarLayerName:
			lbs["layer_name"] = key.LayerName
		case observability.ObserveVarTileX:
			lbs["x"] = strconv.FormatInt(int64(key.X), 10)
		case observability.ObserveVarTileY:
			lbs["y"] = strconv.FormatInt(int64(key.Y), 10)
		case observability.ObserveVarTileZ:
			lbs["z"] = strconv.FormatInt(int64(key.Z), 10)
		}
	}
	lbs["sub_command"] = cmd
	return lbs
}

// Get will record metrics around the getting the tile from the sub cache
func (co *cache) Get(key *tegolaCache.Key) ([]byte, bool, error) {
	co.inFlightGauge.Inc()
	lbs := co.labels("get", key)
	now := time.Now()
	body, ok, err := co.cache.Get(key)
	co.durationSeconds.With(lbs).Observe(time.Since(now).Seconds())
	if err != nil {
		co.errors.With(lbs).Add(1)
		co.inFlightGauge.Dec()
		return body, ok, err
	}
	if ok {
		co.hitsCounter.With(lbs).Add(1)
	} else {
		co.missesCounter.With(lbs).Add(1)
	}

	co.responseSizeBytes.With(lbs).Observe(float64(len(body)))
	co.inFlightGauge.Dec()
	return body, ok, nil
}

// Set will observe metrics around setting the tile via the sub cache.
func (co *cache) Set(key *tegolaCache.Key, body []byte) error {
	co.inFlightGauge.Inc()
	lbs := co.labels("set", key)
	now := time.Now()
	err := co.cache.Set(key, body)
	co.durationSeconds.With(lbs).Observe(time.Since(now).Seconds())
	if err != nil {
		co.errors.With(lbs).Add(1)
		co.inFlightGauge.Dec()
		return err
	}
	co.responseSizeBytes.With(lbs).Observe(float64(len(body)))
	co.inFlightGauge.Dec()
	return nil
}

// Purge will record the metrics around purging the tile from the sub cache.
func (co *cache) Purge(key *tegolaCache.Key) error {
	co.inFlightGauge.Inc()
	lbs := co.labels("purge", key)
	now := time.Now()
	err := co.cache.Purge(key)
	co.durationSeconds.With(lbs).Observe(time.Since(now).Seconds())
	if err != nil {
		co.errors.With(lbs).Add(1)
	}
	co.inFlightGauge.Dec()
	return nil
}

func (co cache) Wrapped() tegolaCache.Interface { return co.cache }
func (co cache) IsObserver() bool               { return true }
