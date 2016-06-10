package mvt

type Linestring []Point

func (line *Linestring) Marshal(p2 Point) []byte {
	var bytes []byte

	for i, point := range *line {
		switch i {
		case 0:
			commandCount := 1
			//	move to
			commandInt := EncodeCommandInt(CommandMoveTo, uint32(commandCount))
			bytes = append(bytes, byte(commandInt))

			//	marshal point relative to provided point
			bytes = append(bytes, point.Marshal(p2)...)
		case 1:
			//	line to with two commands
			commandCount := len(*line) - 1
			commandInt := EncodeCommandInt(CommandLineTo, uint32(commandCount))
			bytes = append(bytes, byte(commandInt))

			//	marshal point relative to last point
			bytes = append(bytes, point.Marshal((*line)[i-1])...)
		default:
			//	marshal point relative to last point
			bytes = append(bytes, point.Marshal((*line)[i-1])...)
		}
	}

	return bytes
}

func (l *Linestring) Type() string {
	return GeoLinestring
}
