// This is inspired by Julien Schmidt's httprouter, in that it uses a patricia tree, but the
// implementation is rather different. Specifically, the routing rules are relaxed so that a
// single path segment may be a wildcard in one route and a static token in another. This gives a
// nice combination of high performance with a lot of convenience in designing the routing patterns.
package httptreemux

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/dimfeld/httppath"
)

// The params argument contains the parameters parsed from wildcards and catch-alls in the URL.
type HandlerFunc func(http.ResponseWriter, *http.Request, map[string]string)
type PanicHandler func(http.ResponseWriter, *http.Request, interface{})

// RedirectBehavior sets the behavior when the router redirects the request to the
// canonical version of the requested URL using RedirectTrailingSlash or RedirectClean.
// The default behavior is to return a 301 status, redirecting the browser to the version
// of the URL that matches the given pattern.
//
// On a POST request, most browsers that receive a 301 will submit a GET request to
// the redirected URL, meaning that any data will likely be lost. If you want to handle
// and avoid this behavior, you may use Redirect307, which causes most browsers to
// resubmit the request using the original method and request body.
//
// Since 307 is supposed to be a temporary redirect, the new 308 status code has been
// proposed, which is treated the same, except it indicates correctly that the redirection
// is permanent. The big caveat here is that the RFC is relatively recent, and older
// browsers will not know what to do with it. Therefore its use is not recommended
// unless you really know what you're doing.
//
// Finally, the UseHandler value will simply call the handler function for the pattern.
type RedirectBehavior int

type PathSource int

const (
	Redirect301 RedirectBehavior = iota // Return 301 Moved Permanently
	Redirect307                         // Return 307 HTTP/1.1 Temporary Redirect
	Redirect308                         // Return a 308 RFC7538 Permanent Redirect
	UseHandler                          // Just call the handler function

	RequestURI PathSource = iota // Use r.RequestURI
	URLPath                      // Use r.URL.Path
)

type TreeMux struct {
	root *node

	Group

	// The default PanicHandler just returns a 500 code.
	PanicHandler PanicHandler

	// The default NotFoundHandler is http.NotFound.
	NotFoundHandler func(w http.ResponseWriter, r *http.Request)

	// Any OPTIONS request that matches a path without its own OPTIONS handler will use this handler,
	// if set, instead of calling MethodNotAllowedHandler.
	OptionsHandler HandlerFunc

	// MethodNotAllowedHandler is called when a pattern matches, but that
	// pattern does not have a handler for the requested method. The default
	// handler just writes the status code http.StatusMethodNotAllowed and adds
	// the required Allowed header.
	// The methods parameter contains the map of each method to the corresponding
	// handler function.
	MethodNotAllowedHandler func(w http.ResponseWriter, r *http.Request,
		methods map[string]HandlerFunc)

	// HeadCanUseGet allows the router to use the GET handler to respond to
	// HEAD requests if no explicit HEAD handler has been added for the
	// matching pattern. This is true by default.
	HeadCanUseGet bool

	// RedirectCleanPath allows the router to try clean the current request path,
	// if no handler is registered for it, using CleanPath from github.com/dimfeld/httppath.
	// This is true by default.
	RedirectCleanPath bool

	// RedirectTrailingSlash enables automatic redirection in case router doesn't find a matching route
	// for the current request path but a handler for the path with or without the trailing
	// slash exists. This is true by default.
	RedirectTrailingSlash bool

	// RemoveCatchAllTrailingSlash removes the trailing slash when a catch-all pattern
	// is matched, if set to true. By default, catch-all paths are never redirected.
	RemoveCatchAllTrailingSlash bool

	// RedirectBehavior sets the default redirect behavior when RedirectTrailingSlash or
	// RedirectCleanPath are true. The default value is Redirect301.
	RedirectBehavior RedirectBehavior

	// RedirectMethodBehavior overrides the default behavior for a particular HTTP method.
	// The key is the method name, and the value is the behavior to use for that method.
	RedirectMethodBehavior map[string]RedirectBehavior

	// PathSource determines from where the router gets its path to search.
	// By default it pulls the data from the RequestURI member, but this can
	// be overridden to use URL.Path instead.
	//
	// There is a small tradeoff here. Using RequestURI allows the router to handle
	// encoded slashes (i.e. %2f) in the URL properly, while URL.Path provides
	// better compatibility with some utility functions in the http
	// library that modify the Request before passing it to the router.
	PathSource PathSource

	// EscapeAddedRoutes controls URI escaping behavior when adding a route to the tree.
	// If set to true, the router will add both the route as originally passed, and
	// a version passed through URL.EscapedPath. This behavior is disabled by default.
	EscapeAddedRoutes bool
}

// Dump returns a text representation of the routing tree.
func (t *TreeMux) Dump() string {
	return t.root.dumpTree("", "")
}

func (t *TreeMux) serveHTTPPanic(w http.ResponseWriter, r *http.Request) {
	if err := recover(); err != nil {
		t.PanicHandler(w, r, err)
	}
}

func (t *TreeMux) redirectStatusCode(method string) (int, bool) {
	var behavior RedirectBehavior
	var ok bool
	if behavior, ok = t.RedirectMethodBehavior[method]; !ok {
		behavior = t.RedirectBehavior
	}
	switch behavior {
	case Redirect301:
		return http.StatusMovedPermanently, true
	case Redirect307:
		return http.StatusTemporaryRedirect, true
	case Redirect308:
		// Go doesn't have a constant for this yet. Yet another sign
		// that you probably shouldn't use it.
		return 308, true
	case UseHandler:
		return 0, false
	default:
		return http.StatusMovedPermanently, true
	}
}

func redirect(w http.ResponseWriter, r *http.Request, newPath string, statusCode int) {
	newURL := url.URL{
		Path:     newPath,
		RawQuery: r.URL.RawQuery,
		Fragment: r.URL.Fragment,
	}
	http.Redirect(w, r, newURL.String(), statusCode)
}

func (t *TreeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if t.PanicHandler != nil {
		defer t.serveHTTPPanic(w, r)
	}

	path := r.RequestURI
	pathLen := len(path)
	if pathLen > 0 && t.PathSource == RequestURI {
		rawQueryLen := len(r.URL.RawQuery)

		if rawQueryLen != 0 || path[pathLen-1] == '?' {
			// Remove any query string and the ?.
			path = path[:pathLen-rawQueryLen-1]
			pathLen = len(path)
		}
	} else {
		// In testing with http.NewRequest,
		// RequestURI is not set so just grab URL.Path instead.
		path = r.URL.Path
		pathLen = len(path)
	}

	trailingSlash := path[pathLen-1] == '/' && pathLen > 1
	if trailingSlash && t.RedirectTrailingSlash {
		path = path[:pathLen-1]
	}
	n, handler, params := t.root.search(r.Method, path[1:])
	if n == nil {
		if t.RedirectCleanPath {
			// Path was not found. Try cleaning it up and search again.
			// TODO Test this
			cleanPath := httppath.Clean(path)
			n, handler, params = t.root.search(r.Method, cleanPath[1:])
			if n == nil {
				// Still nothing found.
				t.NotFoundHandler(w, r)
				return
			} else {
				if statusCode, ok := t.redirectStatusCode(r.Method); ok {
					// Redirect to the actual path
					redirect(w, r, cleanPath, statusCode)
					return
				}
			}
		} else {
			t.NotFoundHandler(w, r)
			return
		}
	}

	if handler == nil {
		if r.Method == "OPTIONS" && t.OptionsHandler != nil {
			handler = t.OptionsHandler
		}

		if handler == nil {
			t.MethodNotAllowedHandler(w, r, n.leafHandler)
			return
		}
	}

	if !n.isCatchAll || t.RemoveCatchAllTrailingSlash {
		if trailingSlash != n.addSlash && t.RedirectTrailingSlash {
			if statusCode, ok := t.redirectStatusCode(r.Method); ok {
				if n.addSlash {
					// Need to add a slash.
					redirect(w, r, path+"/", statusCode)
				} else if path != "/" {
					// We need to remove the slash. This was already done at the
					// beginning of the function.
					redirect(w, r, path, statusCode)
				}
				return
			}
		}
	}

	var paramMap map[string]string
	if len(params) != 0 {
		if len(params) != len(n.leafWildcardNames) {
			// Need better behavior here. Should this be a panic?
			panic(fmt.Sprintf("httptreemux parameter list length mismatch: %v, %v",
				params, n.leafWildcardNames))
		}

		paramMap = make(map[string]string)
		numParams := len(params)
		for index := 0; index < numParams; index++ {
			paramMap[n.leafWildcardNames[numParams-index-1]] = params[index]
		}
	}

	handler(w, r, paramMap)
}

// MethodNotAllowedHandler is the default handler for TreeMux.MethodNotAllowedHandler,
// which is called for patterns that match, but do not have a handler installed for the
// requested method. It simply writes the status code http.StatusMethodNotAllowed.
func MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request,
	methods map[string]HandlerFunc) {

	for m := range methods {
		w.Header().Add("Allow", m)
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

func New() *TreeMux {
	tm := &TreeMux{
		root:                    &node{path: "/"},
		NotFoundHandler:         http.NotFound,
		MethodNotAllowedHandler: MethodNotAllowedHandler,
		HeadCanUseGet:           true,
		RedirectTrailingSlash:   true,
		RedirectCleanPath:       true,
		RedirectBehavior:        Redirect301,
		RedirectMethodBehavior:  make(map[string]RedirectBehavior),
		PathSource:              RequestURI,
		EscapeAddedRoutes:       false,
	}
	tm.Group.mux = tm
	return tm
}
