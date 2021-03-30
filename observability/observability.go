package observability

import (
	"log"
	"net/http"
	"sort"

	"github.com/go-spatial/tegola/dict"
)

// ErrObserverAlreadyExists is returned if an observer try to register to an observer type that has already registered
type ErrObserverAlreadyExists string

func (err ErrObserverAlreadyExists) Error() string {
	return "Observer " + string(err) + " already exists"
}

// ErrObserverIsNotRegistered is returned if a requested observer type has not been already registered
type ErrObserverIsNotRegistered string

func (err ErrObserverIsNotRegistered) Error() string {
	return "Observer " + string(err) + " is not registered"
}

// InitFunc is a function that is used to configure a observer
type InitFunc func(dicter dict.Dicter) (Interface, error)

// Interface
type Interface interface {

	// Handler returns a http.Handler for the metrics route
	Handler(route string) http.Handler

	// Returns the name of observer
	Name() string

	// PublishBuildInfo will send out a piece of metric to represent the build information if it makes sense to do so.
	PublishBuildInfo()

	APIObserver
	ViewerObserver
}

type APIObserver interface {
	// InstrumentedAPIHttpHandler returns an http.Handler that will instrument the given http handler, for the
	// route and method that was given
	InstrumentedAPIHttpHandler(method, route string, handler http.Handler) http.Handler
}

type ViewerObserver interface {
	// InstrumentedViewerHttpHandler returns an http.Handler that will instrument the given http handler, for the
	// route and method that was given
	InstrumentedViewerHttpHandler(method, route string, handler http.Handler) http.Handler
}

// InstrumentAPIHandler is a convenience  function
func InstrumentAPIHandler(method, route string, observer APIObserver, handler http.Handler) (string, string, http.Handler) {
	if observer == nil {
		return method, route, handler
	}
	return method, route, observer.InstrumentedAPIHttpHandler(method, route, handler)
}

// InstrumentViewHandler is a convenience  function
func InstrumentViewerHandler(method, route string, observer ViewerObserver, handler http.Handler) (string, string, http.Handler) {
	if observer == nil {
		return method, route, handler
	}
	return method, route, observer.InstrumentedViewerHttpHandler(method, route, handler)
}

// observers is a list of the current that have been registered with the system
var observers = map[string]InitFunc{
	"none": noneInit,
}

// Register is called by the init functions of the observers
func Register(observerType string, init InitFunc) error {
	if observers == nil {
		observers = make(map[string]InitFunc)
	}

	if _, ok := observers[observerType]; ok {
		return ErrObserverAlreadyExists(observerType)
	}
	observers[observerType] = init
	return nil
}

// Registered returns the set of Registered observers
func Registered() []string {
	obs := make([]string, len(observers))
	for k := range observers {
		obs = append(obs, k)
	}
	sort.Strings(obs)
	return obs
}

// For function returns a configured observer for the given type, and provided config map
func For(observerType string, config dict.Dicter) (Interface, error) {
	// none should always be registered
	if observerType == "" {
		observerType = "none"
	}
	o, ok := observers[observerType]
	if !ok {
		i, _ := noneInit(nil)
		log.Printf("did not find %v", observerType)
		return i, ErrObserverIsNotRegistered(observerType)
	}
	return o(config)
}
