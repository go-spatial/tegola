package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-spatial/cobra"
	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/cmd/internal/register"
	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/draw/svg"
	"github.com/go-spatial/tegola/internal/convert"
	"github.com/go-spatial/tegola/maths/validate"
	"github.com/go-spatial/tegola/mvt"
	"github.com/go-spatial/tegola/provider"
)

var drawCmd = &cobra.Command{
	Use:   "draw",
	Short: "Draw the requested tile or feature",
	Long:  "The draw command will draw out the feature and the various stages of the encoding process.",
	Run:   drawCommand,
}

var drawOutputBaseDir string
var drawOutputFilenameFormat string

func init() {
	drawCmd.Flags().StringVarP(&drawOutputBaseDir, "output", "o", "_svg_files", "Directory to write svg files to.")
	drawCmd.Flags().StringVarP(&drawOutputFilenameFormat, "format", "f", "{{base_dir}}/z{{z}}_x{{x}}_y{{y}}/{{layer_name}}/geo_{{gid}}_{{count}}.{{ext}}", "filename format")
}

type drawFilename struct {
	z, x, y uint
	basedir string
	format  string
	ext     string
}

func (dfn drawFilename) insureFilename(provider string, layer string, gid int, count int) (string, error) {
	r := strings.NewReplacer(
		"{{base_dir}}", dfn.basedir,
		"{{ext}}", dfn.ext,
		"{{layer_name}}", layer,
		"{{provider_name}}", provider,
		"{{gid}}", strconv.FormatInt(int64(gid), 10),
		"{{count}}", strconv.FormatInt(int64(count), 10),
		"{{z}}", strconv.FormatInt(int64(dfn.z), 10),
		"{{x}}", strconv.FormatInt(int64(dfn.x), 10),
		"{{y}}", strconv.FormatInt(int64(dfn.y), 10),
	)
	filename := filepath.Clean(r.Replace(dfn.format))
	basedir := filepath.Dir(filename)
	if err := os.MkdirAll(basedir, 0711); err != nil {
		return "", err
	}
	return filename, nil
}

func (dfn drawFilename) createFile(provider string, layer string, gid int, count int) (string, *os.File, error) {
	fname, err := dfn.insureFilename(provider, layer, gid, count)
	if err != nil {
		return "", nil, err
	}
	file, err := os.Create(fname)
	return fname, file, err
}

func drawCommand(cmd *cobra.Command, args []string) {

	z, x, y, err := parseTileString(zxystr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid zxy (%v): %v\n", zxystr, err)
		os.Exit(1)
	}

	config, err := config.LoadAndValidate(configFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid config (%v): %v\n", configFilename, err)
		os.Exit(1)
	}
	dfn := drawFilename{
		z:       z,
		x:       x,
		y:       y,
		ext:     "svg",
		format:  drawOutputFilenameFormat,
		basedir: drawOutputBaseDir,
	}

	// convert []env.Map -> []dict.Dicter
	provArr := make([]dict.Dicter, len(config.Providers))
	for i := range provArr {
		provArr[i] = config.Providers[i]
	}

	// register providers
	providers, err := register.Providers(provArr)
	if err != nil {
		log.Fatalf("Error loading providers in config(%v): %v\n", configFilename, err)
	}

	prv, lyr := splitProviderLayer(providerString)
	var allprvs []string
	for name := range providers {
		allprvs = append(allprvs, name)
	}
	var prvs = []string{prv}
	// If prv is "" we are going to go through every feature.
	if prv == "" {
		prvs = allprvs
	}
	for _, name := range prvs {
		tiler, ok := providers[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "Skipping  did not find provider %v\n", name)
			fmt.Fprintf(os.Stderr, "known providers: %v\n", strings.Join(allprvs, ", "))
			continue
		}
		var layers []string
		if lyr == "" {
			lysi, err := tiler.Layers()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Skipping error getting layers for provider (%v): %v\n", name, err)
			}
			for _, li := range lysi {
				layers = append(layers, li.Name())
			}
		} else {
			layers = append(layers, lyr)
		}
		if err := drawFeatures(name, tiler, layers, gid, &dfn); err != nil {
			panic(err)
		}
	}
	provider.Cleanup()
}

func drawFeatures(pname string, tiler provider.Tiler, layers []string, gid int, dfn *drawFilename) error {
	ctx := context.Background()
	ttile := tegola.NewTile(dfn.z, dfn.x, dfn.y)
	slippyTile := slippy.NewTile(dfn.z, dfn.x, dfn.y, tegola.DefaultTileBuffer, tegola.WebMercator)
	for _, name := range layers {
		count := 0
		err := tiler.TileFeatures(ctx, name, slippyTile, func(f *provider.Feature) error {
			if gid != -1 && f.ID != uint64(gid) {
				// Skip the feature.
				return nil
			}
			count++
			cursor := mvt.NewCursor(ttile)

			geometry, err := convert.ToTegola(f.Geometry)
			if err != nil {
				return err
			}

			// Scale
			g := cursor.ScaleGeo(geometry)

			// Simplify
			sg := mvt.SimplifyGeometry(g, ttile.ZEpislon(), true)
			pbb, err := ttile.PixelBufferedBounds()
			if err != nil {
				return err
			}

			// Clip and validate
			ext := geom.NewExtent([2]float64{pbb[0], pbb[1]}, [2]float64{pbb[2], pbb[3]})
			vg, err := validate.CleanGeometry(ctx, sg, ext)

			// Draw each of the steps.
			ffname, file, err := dfn.createFile(pname, name, gid, count)
			if err != nil {
				return err
			}
			defer file.Close()

			log.Printf("Writing to file: %v\n", ffname)
			mm := svg.MinMax{0, 0, 4096, 4096}
			mm.OfGeometry(g)
			canvas := &svg.Canvas{
				Board:  mm,
				Region: svg.MinMax{0, 0, 4096, 4096},
			}
			canvas.Init(file, 1440, 900, false)

			canvas.DrawRegion(true)

			canvas.Commentf("MinMax: %v\n", mm)

			log.Println("\tDrawing original version.")
			canvas.DrawGeometry(g, fmt.Sprintf("%v_scaled", gid), "fill-rule:evenodd; fill:yellow;opacity:1", "fill:black", false)

			log.Println("\tDrawing simplified version.")
			canvas.DrawGeometry(sg, fmt.Sprintf("%v_simplifed", gid), "fill-rule:evenodd; fill:green;opacity:0.5", "fill:green;opacity:0.5", false)

			log.Println("\tDrawing clipped version.")
			canvas.DrawGeometry(vg, fmt.Sprintf("clipped_%v", gid), "fill-rule:evenodd; fill:green;opacity:0.5", "fill:green;opacity:0.5", false)

			// Flush the canvas.
			canvas.End()

			return nil
		})
		if err != nil {
			return err
		}
		fmt.Printf("// Number of geometries drawn for %v.%v : %v\n", pname, name, count)
	}
	return nil
}
