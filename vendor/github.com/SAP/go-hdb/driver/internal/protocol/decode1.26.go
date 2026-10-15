//go:build !go1.27

package protocol

import (
	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

func decodeResult(tc typeCode, dec *encoding.Decoder, attrs *ReaderAttrs, lobReader LobReader, scale int) (any, error) { //nolint: gocyclo
	switch tc {
	case tcBoolean:
		return dec.BooleanField()
	case tcTinyint:
		if !dec.Bool() { // null value
			return nil, nil
		}
		return int64(dec.Byte()), nil
	case tcSmallint:
		if !dec.Bool() { // null value
			return nil, nil
		}
		return int64(dec.Int16()), nil
	case tcInteger:
		if !dec.Bool() { // null value
			return nil, nil
		}
		return int64(dec.Int32()), nil
	case tcBigint:
		if !dec.Bool() { // null value
			return nil, nil
		}
		return dec.Int64(), nil
	case tcReal:
		return dec.RealField()
	case tcDouble:
		return dec.DoubleField()
	case tcDate:
		return dec.DateField()
	case tcTime:
		return dec.TimeField()
	case tcTimestamp:
		return dec.TimestampField()
	case tcLongdate:
		return dec.LongdateField()
	case tcSeconddate:
		return dec.SeconddateField()
	case tcDaydate:
		return dec.DaydateField(attrs.emptyDateAsNull)
	case tcSecondtime:
		return dec.SecondtimeField()
	case tcDecimal:
		return dec.DecimalField()
	case tcFixed8:
		return dec.Fixed8Field(scale)
	case tcFixed12:
		return dec.Fixed12Field(scale)
	case tcFixed16:
		return dec.Fixed16Field(scale)
	case tcChar, tcVarchar, tcString, tcBstring, tcBinary, tcVarbinary:
		return dec.VarField()
	case tcAlphanum:
		return dec.AlphanumField(attrs.alphanumDfv1)
	case tcNchar, tcNvarchar, tcNstring, tcShorttext:
		return dec.Cesu8Field()
	case tcStPoint, tcStGeometry:
		return dec.HexField()
	case tcBlob, tcClob, tcLocator, tcBintext:
		return decodeLobOutDescr(dec, nil, lobReader, attrs.lobChunkSize)
	case tcText, tcNclob, tcNlocator:
		return decodeLobOutDescr(dec, dec.Transformer(), lobReader, attrs.lobChunkSize)
	default:
		panic("invalid type code")
	}
}
