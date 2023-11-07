package source

import (
	"strings"
	"testing"

	"github.com/go-spatial/tegola/internal/env"
)

func TestInitSource(t *testing.T) {
	var (
		src ConfigSource
		err error
	)

	_, err = InitSource("invalidtype", env.Dict{}, "")
	if err == nil {
		t.Error("InitSource should error if invalid source type provided; no error returned.")
	}

	_, err = InitSource("file", env.Dict{}, "")
	if err == nil {
		t.Error("InitSource should return error from underlying source type (file) if no directory provided.")
	}

	src, err = InitSource("file", env.Dict{"dir": "config"}, "/tmp")
	if err != nil {
		t.Errorf("Unexpected error from InitSource: %s", err)
	}

	if src.Type() != "file" {
		t.Errorf("Expected source type %s, found %s", "file", src.Type())
	}
}

func TestParseApp(t *testing.T) {
	conf := `
	[[providers]]
	name = "test_postgis"
	type = "mvt_postgis"
	uri = "postgres:/username:password@127.0.0.1:5423/some_db"
	srid = 3857

		[[providers.layers]]
		name = "dynamic"
		sql = "id, ST_AsMVTGeom(wkb_geometry, !BBOX!) as geom FROM some_table WHERE wkb_geometry && !BBOX!"
		geometry_type = "polygon"

	[[maps]]
	name = "stuff"

		[[maps.layers]]
		provider_layer = "test_postgis.dynamic"
		min_zoom = 2
		max_zoom = 18

		[[maps.params]]
		name = "param"
		token = "!PaRaM!"
	`

	r := strings.NewReader(conf)

	// Should load TOML file.
	app, err := parseApp(r, "some_key")
	if err != nil {
		t.Errorf("Unexpected error from parseApp: %s", err)
		return
	}

	if app.Key != "some_key" {
		t.Errorf("Expected app key \"some_key\", found %s", app.Key)
	}

	if len(app.Providers) != 1 {
		t.Error("Failed to load providers from TOML")
	} else {
		name, err := app.Providers[0].String("name", nil)
		if err != nil || name != "test_postgis" {
			t.Errorf("Expected provider name \"test_postgis\", found %s (err=%s)", name, err)
		}
	}

	if len(app.Maps) != 1 {
		t.Error("Failed to load maps from TOML")
	} else if app.Maps[0].Name != "stuff" {
		t.Errorf("Expected map name \"stuff\", found %s", app.Maps[0].Name)
	}

	// Should normalize map params.
	token := "!PARAM!"
	if len(app.Maps) == 1 && app.Maps[0].Parameters[0].Token != token {
		t.Errorf("Expected map query param with token %s, found %s.", token, app.Maps[0].Parameters[0].Token)
	}
}
