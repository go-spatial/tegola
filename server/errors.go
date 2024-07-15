package server

import "fmt"

type ErrMalformedTileTemplateURL struct {
	Got string
}

func (e ErrMalformedTileTemplateURL) Error() string {
	return fmt.Sprintf("Expected URL in the format scheme://domain.tld/maps/:map_name/{z}/{x}/{y}.pbf. Got %s", e.Got)
}
