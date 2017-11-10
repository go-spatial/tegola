package cmd

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/atlas"
)

var (
	cacheXYZ string
	cacheMap string
)

var cacheCmd = &cobra.Command{
	Use:       "cache",
	Short:     "Manipulate the cache",
	Long:      `Use the cache command to pre seed or purge the cache`,
	ValidArgs: []string{"seed", "purge"},
	Args:      cobra.OnlyValidArgs,
}

var cacheSeedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed the cache with pre generated tiles",
	Long:  `Long description comming`,
	Run: func(cmd *cobra.Command, args []string) {
		initConfig()

		var err error
		var maps []atlas.Map
		var tiles []tegola.Tile

		if cacheMap != "" {
			m, err := atlas.GetMap(cacheMap)
			if err != nil {
				log.Fatal(err)
			}

			maps = append(maps, m)
		} else {
			maps = atlas.AllMaps()
		}

		if atlas.GetCache() == nil {
			log.Fatal("mising cache backend. check your config")
		}

		if cacheXYZ != "" {
			t, err := parseTileString(cacheXYZ)
			if err != nil {
				log.Fatal(err)
			}

			tiles = append(tiles, t)
		}

		//	iterate maps and tiles
		for i := range maps {
			for j := range tiles {
				if err = maps[i].SeedTile(tiles[j]); err != nil {
					log.Fatal("error seeding tile: %v", err)
				}
			}
		}
	},
}

var cachePurgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Purge the cache in various ways",
	Long:  `Long description comming`,
	Run: func(cmd *cobra.Command, args []string) {
		initConfig()

		var maps []atlas.Map
		var tiles []tegola.Tile

		if cacheMap != "" {
			m, err := atlas.GetMap(cacheMap)
			if err != nil {
				log.Fatal(err)
			}

			maps = append(maps, m)
		} else {
			maps = atlas.AllMaps()
		}

		if atlas.GetCache() == nil {
			log.Fatal("mising cache backend. check your config")
		}

		if cacheXYZ != "" {
			t, err := parseTileString(cacheXYZ)
			if err != nil {
				log.Fatal(err)
			}

			tiles = append(tiles, t)
		}

		//	iterate maps and tiles
		for i := range maps {
			for j := range tiles {
				if err := maps[i].PurgeTile(tiles[j]); err != nil {
					log.Fatal("error seeding tile: %v", err)
				}
			}
		}
	},
}

//	parseTileString converts a Z/X/Y formatted string into a tegola tile
func parseTileString(str string) (tegola.Tile, error) {
	var tile tegola.Tile

	parts := strings.Split(cacheXYZ, "/")
	if len(parts) != 3 {
		return tile, fmt.Errorf("invalid zxy value (%v). expecting the format z/x/y", cacheXYZ)
	}

	z, err := strconv.Atoi(parts[0])
	if err != nil {
		return tile, fmt.Errorf("invalid Z value (%v)", z)
	}
	if z < 0 {
		return tile, fmt.Errorf("negative zoom levels are not allowed")
	}

	x, err := strconv.Atoi(parts[1])
	if err != nil {
		return tile, fmt.Errorf("invalid X value (%v)", x)
	}

	y, err := strconv.Atoi(parts[2])
	if err != nil {
		return tile, fmt.Errorf("invalid Y value (%v)", y)
	}

	tile = tegola.Tile{
		Z: z,
		X: x,
		Y: y,
	}

	return tile, nil
}
