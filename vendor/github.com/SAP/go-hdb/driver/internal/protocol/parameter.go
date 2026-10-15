package protocol

import (
	"database/sql/driver"
	"fmt"
	"math/bits"
	"reflect"
	"slices"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

type parameterOptions int8

const (
	poMandatory parameterOptions = 0x01
	poOptional  parameterOptions = 0x02
	poDefault   parameterOptions = 0x04
)

const (
	poMandatoryText = "mandatory"
	poOptionalText  = "optional"
	poDefaultText   = "default"
)

func (k parameterOptions) String() string {
	var s []string
	if k&poMandatory != 0 {
		s = append(s, poMandatoryText)
	}
	if k&poOptional != 0 {
		s = append(s, poOptionalText)
	}
	if k&poDefault != 0 {
		s = append(s, poDefaultText)
	}
	return fmt.Sprintf("%v", s)
}

// ParameterMode represents the parameter mode set.
type ParameterMode int8

// ParameterMode constants.
const (
	pmIn    ParameterMode = 0x01
	pmInout ParameterMode = 0x02
	pmOut   ParameterMode = 0x04
)

const (
	pmInText    = "in"
	pmInoutText = "inout"
	pmOutText   = "out"
)

func (k ParameterMode) String() string {
	var s []string
	if k&pmIn != 0 {
		s = append(s, pmInText)
	}
	if k&pmInout != 0 {
		s = append(s, pmInoutText)
	}
	if k&pmOut != 0 {
		s = append(s, pmOutText)
	}
	return fmt.Sprintf("%v", s)
}

// ParameterField contains database field attributes for parameters.
type ParameterField struct {
	names            *fieldNames
	ofs              int // field name offset & used for index in case of tableRef or tableRows type
	prec             int // length
	scale            int // fraction
	parameterOptions parameterOptions
	tc               typeCode
	mode             ParameterMode
}

// NewTableRowsParameterField returns a ParameterField representing table rows.
func NewTableRowsParameterField(idx int) *ParameterField {
	return &ParameterField{ofs: idx, tc: TcTableRows, mode: pmOut}
}

func (f *ParameterField) fieldName() string {
	switch f.tc {
	case TcTableRows:
		return fmt.Sprintf("table %d", f.ofs)
	default:
		return f.names.name(uint32(f.ofs)) //nolint: gosec
	}
}

func (f *ParameterField) isNullable() bool { return f.parameterOptions == poOptional }

func (f *ParameterField) String() string {
	return fmt.Sprintf("parameterOptions %s typeCode %s mode %s precision %d scale %d name %s",
		f.parameterOptions,
		f.tc,
		f.mode,
		f.prec,
		f.scale,
		f.fieldName(),
	)
}

// IsLob returns true if the ParameterField is of type lob, false otherwise.
func (f *ParameterField) IsLob() bool { return f.tc.isLob() }

// Convert returns the result of the fieldType conversion.
func (f *ParameterField) Convert(v any) (any, error) {
	cv, err := convertField(f.tc, v, f.prec, f.scale)
	if err != nil {
		return nil, fmt.Errorf("field %[1]s type code %[2]s type %[3]T value %[3]v conversion error %[4]w", f.fieldName(), f.tc, v, err)
	}
	return cv, nil
}

// DatabaseTypeName returns the type name of the field.
// It implements the go-hdb driver ColumnType interface.
func (f *ParameterField) DatabaseTypeName() string { return f.tc.typeName() }

// DecimalSize returns the type precision and scale of the field.
// It implements the go-hdb driver ColumnType interface.
func (f *ParameterField) DecimalSize() (int64, int64, bool) {
	if f.tc.isDecimalType() {
		return int64(f.prec), int64(f.scale), true
	}
	return 0, 0, false
}

// Length returns the type length of the field.
// It implements the go-hdb driver ColumnType interface.
func (f *ParameterField) Length() (int64, bool) {
	if f.tc.isVariableLength() {
		return int64(f.prec), true
	}
	return 0, false
}

// Name returns the parameter field name.
// It implements the go-hdb driver ColumnType interface.
func (f *ParameterField) Name() string { return f.fieldName() }

// Nullable returns true if the field may be null, false otherwise.
// It implements the go-hdb driver ColumnType interface.
func (f *ParameterField) Nullable() (bool, bool) { return f.isNullable(), true }

// ScanType returns the scan type of the field.
// It implements the go-hdb driver ColumnType interface.
func (f *ParameterField) ScanType() reflect.Type { return f.tc.dataType().ScanType(f.isNullable()) }

// In returns true if the parameter field is an input field.
// It implements the go-hdb driver ParameterType interface.
func (f *ParameterField) In() bool { return f.mode == pmInout || f.mode == pmIn }

// Out returns true if the parameter field is an output field.
// It implements the go-hdb driver ParameterType interface.
func (f *ParameterField) Out() bool { return f.mode == pmInout || f.mode == pmOut }

// InOut returns true if the parameter field is an input/output field.
// It implements the go-hdb driver ParameterType interface.
func (f *ParameterField) InOut() bool { return f.mode == pmInout }

func (f *ParameterField) decode(dec *encoding.Decoder) {
	f.parameterOptions = parameterOptions(dec.Int8())
	f.tc = typeCode(dec.Byte())
	f.mode = ParameterMode(dec.Int8())
	dec.Skip(1) // filler
	f.ofs = int(dec.Uint32())
	f.prec = int(dec.Int16())
	f.scale = int(dec.Int16())
	dec.Skip(4)                      // filler
	f.names.insertOfs(uint32(f.ofs)) //nolint: gosec
}

func (f *ParameterField) encodePrm(enc *encoding.Encoder, v any) error {
	encTc := f.tc.encTc()
	if v == nil && f.tc.supportNullValue() {
		enc.Byte(byte(f.tc.nullValue())) // null value type code
		return nil
	}
	enc.Byte(byte(encTc)) // type code
	switch f.tc {
	case tcBoolean:
		return enc.BooleanField(v)
	case tcTinyint:
		return enc.TinyintField(v)
	case tcSmallint:
		return enc.SmallintField(v)
	case tcInteger:
		return enc.IntegerField(v)
	case tcBigint:
		return enc.BigintField(v)
	case tcReal:
		return enc.RealField(v)
	case tcDouble:
		return enc.DoubleField(v)
	case tcDate:
		return enc.DateField(v)
	case tcTime:
		return enc.TimeField(v)
	case tcTimestamp:
		return enc.TimestampField(v)
	case tcLongdate:
		return enc.LongdateField(v)
	case tcSeconddate:
		return enc.SeconddateField(v)
	case tcDaydate:
		return enc.DaydateField(v)
	case tcSecondtime:
		return enc.SecondtimeField(v)
	case tcDecimal:
		return enc.DecimalField(v)
	case tcFixed8:
		return enc.FixedField(v, 8)
	case tcFixed12:
		return enc.FixedField(v, 12)
	case tcFixed16:
		return enc.FixedField(v, 16)
	case tcChar, tcVarchar, tcString, tcBstring, tcAlphanum, tcBinary, tcVarbinary:
		return enc.VarField(v)
	case tcNchar, tcNvarchar, tcNstring, tcShorttext:
		return enc.Cesu8Field(v)
	case tcStPoint, tcStGeometry:
		return enc.HexField(v)
	case tcBlob, tcClob, tcLocator, tcNclob, tcText, tcNlocator, tcBintext:
		descr, ok := v.(*LobInDescr)
		if !ok {
			panic("invalid lob value") // should never happen
		}
		enc.Byte(byte(descr.opt))    //nolint: gosec
		enc.Int32(int32(descr.size)) //nolint: gosec
		enc.Int32(int32(descr.pos))  //nolint: gosec
		return nil
	default:
		panic(fmt.Errorf("invalid type code %[1]d %[1]s", f.tc)) // should never happen
	}
}

// decodeParameter decodes a single parameter value. The type code is the
// first byte (see encodePrm). A high bit set on the type code byte signals a
// null value.
func (f *ParameterField) decodeParameter(dec *encoding.Decoder, attrs *ReaderAttrs) (any, error) {
	tc := typeCode(dec.Byte())
	if tc&0x80 != 0 { // high bit set -> null value
		return nil, nil
	}
	return decodeParameter(f.tc, dec, attrs)
}

// ParameterMetadata represents the metadata of a parameter.
type ParameterMetadata struct {
	ParameterFields []*ParameterField
}

func (m *ParameterMetadata) String() string {
	return fmt.Sprintf("parameter %v", m.ParameterFields)
}

func (m *ParameterMetadata) decode(dec *encoding.Decoder, header *PartHeader, attrs *ReaderAttrs) error {
	m.ParameterFields = make([]*ParameterField, header.numArg())
	names := &fieldNames{}
	for i := range len(m.ParameterFields) {
		f := &ParameterField{names: names}
		f.decode(dec)
		m.ParameterFields[i] = f
	}
	if err := names.decode(dec); err != nil {
		return err
	}
	return nil
}

// InputParameters represents the set of input parameters.
type InputParameters struct {
	InputFields []*ParameterField
	nvargs      []driver.NamedValue
	FieldValues []driver.Value
}

// NewInputParameters returns an InputParameters instance.
func NewInputParameters(inputFields []*ParameterField, nvargs []driver.NamedValue) *InputParameters {
	return &InputParameters{InputFields: inputFields, nvargs: nvargs}
}

func (p *InputParameters) String() string {
	if len(p.FieldValues) != 0 {
		return fmt.Sprintf("fields %s values %v", p.InputFields, p.FieldValues)
	}
	return fmt.Sprintf("fields %s len(args) %d args %v", p.InputFields, len(p.nvargs), p.nvargs)
}

func (p *InputParameters) numArg() int {
	numColumns := len(p.InputFields)
	if numColumns == 0 { // avoid divide-by-zero (e.g. prepare without parameters)
		return 0
	}
	return len(p.nvargs) / numColumns
}

func (p *InputParameters) decode(dec *encoding.Decoder, header *PartHeader, attrs *ReaderAttrs) error {
	numColumns := len(p.InputFields)
	if numColumns == 0 { // cannot decode without input field metadata
		return nil
	}
	numRows := header.numArg() // number of argument rows
	if numRows < 0 {
		return fmt.Errorf("invalid number of arguments %d", numRows)
	}
	if hi, _ := bits.Mul(uint(numRows), uint(numColumns)); hi != 0 {
		return fmt.Errorf("arguments too large: %d rows x %d cols", numRows, numColumns)
	}

	n := numRows * numColumns
	p.FieldValues = slices.Grow(p.FieldValues, n)[:0]

	for range numRows {
		start := len(p.FieldValues)
		for _, f := range p.InputFields {
			v, err := f.decodeParameter(dec, attrs)
			if err != nil {
				return err
			}
			p.FieldValues = append(p.FieldValues, v)
		}
		// lob first chunks follow the row descriptors on the wire
		for _, v := range p.FieldValues[start:] {
			if descr, ok := v.(*LobInDescr); ok {
				descr.buf.Write(dec.Bytes(descr.size))
			}
		}
	}
	return nil
}

func (p *InputParameters) encode(enc *encoding.Encoder) error {
	numColumns := len(p.InputFields)
	if numColumns == 0 { // avoid divide-by-zero (e.g. prepare without parameters)
		return nil
	}

	for i := range len(p.nvargs) / numColumns { // row-by-row
		hasInLob := false

		for j := range numColumns {
			// mass insert
			f := p.InputFields[j]
			if err := f.encodePrm(enc, p.nvargs[i*numColumns+j].Value); err != nil {
				return err
			}
			if f.IsLob() && f.In() {
				hasInLob = true
			}
		}
		// lob input parameter: write first data chunk
		if hasInLob {
			for j := range numColumns {
				if lobInDescr, ok := p.nvargs[i*numColumns+j].Value.(*LobInDescr); ok {
					lobInDescr.writeFirst(enc)
				}
			}
		}
	}
	return nil
}

// OutputParameters represents the set of output parameters.
type OutputParameters struct {
	OutputFields []*ParameterField
	FieldValues  []driver.Value
	DecodeErrors DecodeErrors
}

func (p *OutputParameters) String() string {
	return fmt.Sprintf("fields %v values %v", p.OutputFields, p.FieldValues)
}
