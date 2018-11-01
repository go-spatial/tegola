package cache

import (
	"fmt"

	"github.com/spf13/cobra"
)

func setupMinMaxZoomFlags(cmd *cobra.Command, min, max uint) {
	cmd.Flags().UintVarP(&minZoom, "min-zoom", "", min, "min zoom to seed cache from.")
	cmd.Flags().UintVarP(&maxZoom, "max-zoom", "", max, "max zoom to seed cache to")
}

func IsMinMaxZoomExplicit(cmd *cobra.Command) bool {
	return !(cmd.Flag("min-zoom").Changed || cmd.Flag("max-zoom").Changed)
}

func minMaxZoomValidate(cmd *cobra.Command, args []string) (err error) {
	zooms, err = sliceFromRange(minZoom, maxZoom)
	if err != nil {
		return fmt.Errorf("invalid zoom range, %v", err)
	}
	return nil
}

func setupTileNameFormat(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&tileListFormat, "format", "", "/zxy", "4 character string where the first character is a non-numeric delimiter followed by 'z', 'x' and 'y' defining the coordinate order")
}

func tileNameFormatValidate(cmd *cobra.Command, args []string) (err error) {
	format, err = NewFormat(tileListFormat)
	return err
}
