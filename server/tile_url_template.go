package server

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"
)

const (
	TileURLMapsToken  = "maps"
	TileURLZToken     = "{z}"
	TileURLXToken     = "{x}"
	TileURLYToken     = "{y}"
	TileURLFileFormat = "pbf"
)

// TileURLTemplate is responsible for forming up tile URLs which
// contain the uri template variables {z}, {x} and {y}.pbf as the suffix
// of the path.
type TileURLTemplate struct {
	Scheme     string
	Host       string
	PathPrefix string
	Query      url.Values
	MapName    string
	LayerName  string
}

func (t *TileURLTemplate) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.String() + `"`), nil
}

func (t *TileURLTemplate) UnmarshalJSON(data []byte) error {
	var urlStr string
	if err := json.Unmarshal(data, &urlStr); err != nil {
		return err
	}

	urlParsed, err := url.Parse(urlStr)
	if err != nil {
		return err
	}

	t.Scheme = urlParsed.Scheme
	t.Host = urlParsed.Host

	query := urlParsed.Query()
	if len(query) != 0 {
		t.Query = query
	}

	// split the path into parts
	pathParts := strings.Split(urlParsed.Path, "/")

	var foundZToken bool
	for i := range pathParts {
		// loop the path parts until we find the "maps" URL token
		if pathParts[i] == TileURLMapsToken {
			// check pathParts length before inspecting further
			// ahead in the slice
			if len(pathParts) < i+1 {
				return ErrMalformedTileTemplateURL{
					Got: urlStr,
				}
			}
			// value after the maps token is the map name
			t.MapName = pathParts[i+1]

			// path prefix is everything before the maps token
			pathPrefix := pathParts[0:i]
			if len(pathPrefix) != 0 {
				t.PathPrefix = path.Join(pathPrefix...)
			}

			continue
		}

		if pathParts[i] == TileURLZToken {
			foundZToken = true
			// value before the z token is either the
			// map name or the layer name.
			if pathParts[i-1] != t.MapName {
				t.LayerName = pathParts[i-1]
			}
			break
		}
	}
	if !foundZToken {
		return ErrMalformedTileTemplateURL{
			Got: urlStr,
		}
	}

	return nil
}

func (t TileURLTemplate) String() string {
	query := t.Query.Encode()
	if query != "" {
		// prepend our query identifier
		query = "?" + query
	}

	// usually the url.URL struct would be used for building the URL, but what's being
	// built here is a "uri template" that contains the placeholders: {z}/{x}/{y}, which
	// will be encoded when calling String() on url.URL. URI templates are described in detail
	// at https://datatracker.ietf.org/doc/html/rfc6570.
	return fmt.Sprintf("%s://%s%s%s", t.Scheme, t.Host, t.Path(), query)
}

// Path will build the path part of the URI, including the template variables
// {z}, {x} and {y}.pbf. The path will start with a forward slash ("/").
func (t TileURLTemplate) Path() string {
	pathParts := []string{}

	if t.PathPrefix != "" {
		pathParts = append(pathParts, t.PathPrefix)
	}
	// add the maps part of the path
	pathParts = append(pathParts, TileURLMapsToken, t.MapName)

	// if we have a layer add layer parts to the path
	if t.LayerName != "" {
		pathParts = append(pathParts, t.LayerName)
	}

	// add the z/x/y uri template to the path
	pathParts = append(pathParts, TileURLZToken, TileURLXToken, TileURLYToken+"."+TileURLFileFormat)

	// build the path
	return path.Clean("/" + path.Join(pathParts...))
}
