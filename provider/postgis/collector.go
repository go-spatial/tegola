package postgis

import (
	"strings"

	"github.com/go-spatial/tegola/observability"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

type connectionPoolCollector struct {
	*pgxpool.Pool

	// providerName the pool is created for
	// required to make metrics unique
	providerName string

	maxConnectionDesc        *prometheus.Desc
	currentConnectionsDesc   *prometheus.Desc
	availableConnectionsDesc *prometheus.Desc
}

func (c connectionPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, ch)
}

func (c connectionPoolCollector) Collect(ch chan<- prometheus.Metric) {
	if c.Pool == nil {
		return
	}
	stat := c.Stat()

	ch <- prometheus.MustNewConstMetric(
		c.maxConnectionDesc,
		prometheus.GaugeValue,
		float64(stat.MaxConns()),
	)
	ch <- prometheus.MustNewConstMetric(
		c.currentConnectionsDesc,
		prometheus.GaugeValue,
		float64(stat.AcquiredConns()),
	)
	ch <- prometheus.MustNewConstMetric(
		c.availableConnectionsDesc,
		prometheus.GaugeValue,
		float64(stat.MaxConns()-stat.AcquiredConns()),
	)
}

func (c *connectionPoolCollector) Collectors(
	prefix string,
	_ func(configKey string) map[string]any,
) ([]observability.Collector, error) {
	if c == nil {
		return nil, nil
	}
	if prefix != "" && !strings.HasSuffix(prefix, "_") {
		prefix = prefix + "_"
	}

	// a constant label ensures that the metrics are unique
	// this allows the registration of multiple providers in the same
	// config.
	c.maxConnectionDesc = prometheus.NewDesc(
		prefix+"postgres_max_connections",
		"Max number of postgres connections in the pool",
		nil,
		prometheus.Labels{"provider_name": c.providerName},
	)

	c.currentConnectionsDesc = prometheus.NewDesc(
		prefix+"postgres_current_connections",
		"Current number of postgres connections in the pool",
		nil,
		prometheus.Labels{"provider_name": c.providerName},
	)

	c.availableConnectionsDesc = prometheus.NewDesc(
		prefix+"postgres_available_connections",
		"Current number of available postgres connections in the pool",
		nil,
		prometheus.Labels{"provider_name": c.providerName},
	)

	return []observability.Collector{c}, nil
}
