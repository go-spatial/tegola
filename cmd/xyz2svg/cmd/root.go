package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-spatial/tegola/provider"
	"github.com/spf13/cobra"

	_ "github.com/go-spatial/tegola/provider/postgis"
)

var Root = &cobra.Command{
	Use:   "tegola-tool",
	Short: "tegola-tool is a tool to help debug the tegol server and libraries.",
	Long:  "tegola-tool is a tool to help debug the tegola server and libraries.",
}

var (
	// configFilename is the name of the config file.
	configFilename string
	// zxystr tile to work on.
	zxystr string
	// provider is the provider that we are going to be quering.
	providerString string
	// the gid of the feature to be selected.
	gid int

	// Z,X,Y for the tile
	Z, X, Y uint
	// Providers that were in the config file.
	Providers map[string]provider.Tiler
)

func init() {
	// Config file
	Root.PersistentFlags().StringVarP(&configFilename, "config", "c", "config.toml", "path to config file")
	Root.PersistentFlags().StringVarP(&zxystr, "tile", "t", "", "tile in z/x/y format")
	Root.PersistentFlags().StringVarP(&providerString, "provider", "p", "", "provider in the format: “$provider.$layer”")
	Root.PersistentFlags().IntVarP(&gid, "gid", "g", -1, "the feature to select.")

	Root.AddCommand(drawCmd)
}

//initProviders will return a map of registered providers in the config file.
func initProviders(providers []map[string]interface{}) (prvs map[string]provider.Tiler, err error) {

	prvs = make(map[string]provider.Tiler)

	// iterate providers
	for _, p := range providers {
		// lookup our provider name
		n, ok := p["name"]
		if !ok {
			return prvs, errors.New("missing 'name' parameter for provider")
		}

		pname, found := n.(string)
		if !found {
			return prvs, fmt.Errorf("'name' or provider must be of type string")
		}

		// check if a proivder with this name is alrady registered
		if _, ok := prvs[pname]; ok {
			return prvs, fmt.Errorf("provider (%v) already registered!", pname)
		}

		// lookup our provider type
		t, ok := p["type"]
		if !ok {
			return prvs, fmt.Errorf("missing 'type' parameter for provider (%v)", pname)
		}

		ptype, found := t.(string)
		if !found {
			return prvs, fmt.Errorf("'type' for provider (%v) must be a string", pname)
		}

		// register the provider
		prov, err := provider.For(ptype, p)
		if err != nil {
			return prvs, err
		}
		// add the provider to our map of registered providers
		prvs[pname] = prov
	}
	return prvs, err
}

//parseTileString will convert a z/x/y formatted string into a the three components.
func parseTileString(str string) (uint, uint, uint, error) {
	parts := strings.Split(str, "/")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid zxy value “%v”; expected format “z/x/y”", str)
	}
	attr := [3]string{"z", "x", "y"}
	var vals [3]uint
	var placeholder uint64
	var err error
	for i := range attr {

		placeholder, err = strconv.ParseUint(parts[i], 10, 64)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid %v value (%v); should be a postive integer.", attr[i], vals[i])
		}
		vals[i] = uint(placeholder)
	}
	return vals[0], vals[1], vals[2], nil

}

//splitProviderLayer will convert a “$provider.$layer” formatted string into a the two components.
func splitProviderLayer(providerLayer string) (provider, layer string) {
	parts := strings.SplitN(providerLayer, ".", 2)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	default:
		return parts[0], parts[1]
	}
}
