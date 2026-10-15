package protocol

import (
	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

func decodeLobParameter(d *encoding.Decoder) (any, error) {
	opt := LobOptions(d.Int8())
	size := int(d.Int32())
	pos := int(d.Int32())
	return &LobInDescr{opt: opt, pos: pos, size: size}, nil
}

func decodeParameter(tc typeCode, d *encoding.Decoder, attrs *ReaderAttrs) (any, error) {
	switch tc {
	case tcBoolean:
		return d.BooleanField()
	case tcTinyint:
		return int64(d.Byte()), nil
	case tcSmallint:
		return int64(d.Int16()), nil
	case tcInteger:
		return int64(d.Int32()), nil
	case tcBigint:
		return d.Int64(), nil
	case tcReal:
		return d.RealField()
	case tcDouble:
		return d.DoubleField()
	case tcDate:
		return d.DateField()
	case tcTime:
		return d.TimeField()
	case tcTimestamp:
		return d.TimestampField()
	case tcLongdate:
		return d.LongdateField()
	case tcSeconddate:
		return d.SeconddateField()
	case tcDaydate:
		return d.DaydateField(attrs.emptyDateAsNull)
	case tcSecondtime:
		return d.SecondtimeField()
	case tcDecimal:
		return d.DecimalField()
	case tcFixed8:
		return d.Fixed(8), nil
	case tcFixed12:
		return d.Fixed(12), nil
	case tcFixed16:
		return d.Fixed(16), nil
	case tcChar, tcVarchar, tcString, tcBstring, tcBinary, tcVarbinary:
		return d.VarField()
	case tcAlphanum:
		return d.AlphanumField(attrs.alphanumDfv1)
	case tcNchar, tcNvarchar, tcNstring, tcShorttext:
		return d.Cesu8Field()
	case tcStPoint, tcStGeometry:
		return d.HexField()
	case tcBlob, tcClob, tcLocator, tcBintext:
		return decodeLobParameter(d)
	case tcText, tcNclob, tcNlocator:
		return decodeLobParameter(d)
	default:
		panic("invalid type code")
	}
}
