package prometheus

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-spatial/tegola/internal/build"
	"github.com/prometheus/client_golang/prometheus/push"

	"github.com/go-spatial/tegola/internal/p"

	tegolaCache "github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/observability"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type byteSize uint64

const (
	b  byteSize = 1
	kb byteSize = 1 << (10 * iota)
	mb
	gb
)

const (
	Name = "prometheus"

	httpAPI    = "tegola_api"
	httpViewer = "tegola_viewer"
)

func init() {
	err := observability.Register(Name, New, cleanUp)
	if err != nil {
		panic(err)
	}
}

type observer struct {
	// URLPrefix is the server's prefix
	URLPrefix string

	// observeVars are the vars (:foo) in a url that should be turned into a label
	// Default values for this via new is []string{":map_name",":layer_name",":z"}
	observeVars []string

	httpHandlers map[string]*httpHandler
	registry     prometheus.Registerer

	publishedBuildInfo sync.Once
	initCall           sync.Once
	pushURL            string
	pushCadenceSeconds int
	pushCleanupFuncIdx int
}

func New(config dict.Dicter) (observability.Interface, error) {
	// We don't have anything for now for the config
	var obs observer
	obs.registry = prometheus.DefaultRegisterer
	obs.httpHandlers = make(map[string]*httpHandler)
	obs.pushCleanupFuncIdx = -1

	// Are we pushing our metrics?
	pushURL, _ := config.String("push_url", nil)
	if pushURL != "" {
		obs.pushURL = pushURL
		obs.pushCadenceSeconds, _ = config.Int("push_cadence", p.Int(10))
	}

	obs.observeVars, _ = config.StringSlice("variables")
	if len(obs.observeVars) == 0 {
		obs.observeVars = []string{":map_name", ":layer_name", ":z"}
	}

	NewBuildInfo(obs.registry)

	return &obs, nil
}

func (*observer) Name() string { return Name }

func (observer) Handler(string) http.Handler { return promhttp.Handler() }
func (obs *observer) Init()                  { obs.initCall.Do(obs.init) }
func (obs *observer) init() {
	obs.PublishBuildInfo()
	if obs == nil || obs.pushURL == "" {
		return
	}

	// Start up the push
	// we need to setup a clean up routine to push the metrics when we are shutting down.
	pusher := push.New(obs.pushURL, strings.Join(build.Commands, "_")).Gatherer(prometheus.DefaultGatherer)

	var (
		wg            sync.WaitGroup
		ticker        *time.Ticker
		cancel        context.CancelFunc
		ctx           context.Context
		errorReported bool
	)

	if obs.pushCadenceSeconds > 0 {
		ticker = time.NewTicker(time.Duration(obs.pushCadenceSeconds) * time.Second)
		ctx, cancel = context.WithCancel(context.Background())
		wg.Add(1)
		go func() {
			// start up our cadence
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := pusher.Add(); err != nil && !errorReported {
						log.Errorf("could not push to Pushgateway (%v): %v", obs.pushURL, err)
						errorReported = true
					}
				}
			}
		}()
	}

	cleanUpFunctionsLck.Lock()
	obs.pushCleanupFuncIdx = len(cleanUpFunctions)
	cleanUpFunctions = append(cleanUpFunctions, func() {
		if ticker != nil {
			ticker.Stop()
		}
		if cancel != nil {
			cancel()
		}
		if err := pusher.Add(); err != nil {
			log.Errorf("could not push to Pushgateway (%v): %v", obs.pushURL, err)
		}
		wg.Wait()
	})
	cleanUpFunctionsLck.Unlock()
}

func (obs *observer) Shutdown() {
	if obs.pushCleanupFuncIdx < 0 {
		return
	}
	cleanUpFunctionsLck.Lock()
	cleanUpFunctions[obs.pushCleanupFuncIdx]()
	cleanUpFunctions[obs.pushCleanupFuncIdx] = nil
	obs.pushCleanupFuncIdx = -1
	cleanUpFunctionsLck.Unlock()
}

func (obs *observer) MustRegister(collectors ...observability.Collector) {
	obs.registry.MustRegister(collectors...)
}

func (_ *observer) CollectorConfig(_ string) map[string]interface{} {
	return make(map[string]interface{})
}

func (obs *observer) PublishBuildInfo() { obs.publishedBuildInfo.Do(PublishBuildInfo) }

func (obs *observer) InstrumentedAPIHttpHandler(method, route string, next http.Handler) http.Handler {
	if obs == nil {
		return next
	}
	handler := obs.httpHandlers[httpAPI]
	if handler == nil {
		// need to initialize the handler
		handler = newHttpHandler(obs.registry, httpAPI, obs.URLPrefix, obs.observeVars)
		obs.httpHandlers[httpAPI] = handler
	}
	return handler.InstrumentedHttpHandler(method, route, next)
}

func (obs *observer) InstrumentedViewerHttpHandler(method, route string, next http.Handler) http.Handler {
	if obs == nil {
		return next
	}
	handler := obs.httpHandlers[httpViewer]
	if handler == nil {
		// need to initialize the handler
		handler = newHttpHandler(obs.registry, httpViewer, obs.URLPrefix, obs.observeVars)
		obs.httpHandlers[httpViewer] = handler
	}
	return handler.InstrumentedHttpHandler(method, route, next)
}

func (obs *observer) InstrumentedCache(cacheObject tegolaCache.Interface) tegolaCache.Interface {
	if obs == nil {
		// if we are nil assume no metrics recording is going to happen
		return cacheObject
	}
	return newCache(obs.registry, "tegola_cache", obs.observeVars, cacheObject)
}

var (
	cleanUpFunctionsLck sync.Mutex
	cleanUpFunctions    []func()
)

func cleanUp() {
	cleanUpFunctionsLck.Lock()
	for i := range cleanUpFunctions {
		if cleanUpFunctions[i] != nil {
			cleanUpFunctions[i]()
			cleanUpFunctions[i] = nil
		}
	}
	cleanUpFunctions = cleanUpFunctions[:0]
	cleanUpFunctionsLck.Unlock()
}
