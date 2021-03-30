package prometheus

import (
	"strings"

	"github.com/go-spatial/tegola/internal/build"
	"github.com/prometheus/client_golang/prometheus"
)

var BuildInfo *prometheus.GaugeVec

// NewBuildInfo will create a Gauge that can be used to join other metrics to correlate with the command that produced the
// metric.
// see: https://www.robustperception.io/exposing-the-software-version-to-prometheus
// for the reasoning behind having a build_info item.
func NewBuildInfo(registry prometheus.Registerer) {
	if BuildInfo == nil {
		BuildInfo = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "tegola_build_info",
				Help: "Build information",
			},
			[]string{
				// command is the command line; e.g. tegola serve
				"command",
				// version is the version string
				"version",
				// tags are the build tags the were used
				"tags",
				// revision is the git revision at time of build
				"revision",
				// branch is the git branch at time of build
				"branch",
			},
		)
	}
	registry.MustRegister(BuildInfo)
}

func PublishBuildInfo() {
	BuildInfo.With(prometheus.Labels{
		"command":  strings.Join(build.Commands, " "),
		"version":  build.Version,
		"tags":     strings.Join(build.OrderedTags(), ","),
		"revision": build.GitRevision,
		"branch":   build.GitBranch,
	}).Set(1)
}
