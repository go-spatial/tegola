package features

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
)

const (
	CRS84                 = "http://www.opengis.net/def/crs/OGC/1.3/CRS84"
	CRS84h                = provider.CRS84h
	MaxCRSURIBytes        = 256
	MaxCRSDefinitionBytes = 65536
	MaxCollectionCRSs     = 256
)

// CRSAxis describes a public coordinate axis in wire order.
type CRSAxis struct{ Name, Unit string }

// CRSDefinition describes a locally resolved CRS; it contains no runtime SRID.
type CRSDefinition struct {
	URI, Definition, Datum, SHA256 string
	Axes                           []CRSAxis
	Dimension                      int
	VerticalCRS                    string
}

// CRS owns canonical projection state independent of the mutable global registry.
type CRS struct {
	definition          CRSDefinition
	srid                uint64
	geographic, swapped bool
	projection          *crsconfig.FeatureProjection
}

func (c CRS) URI() string          { return c.definition.URI }
func (c CRS) Dimension() int       { return c.definition.Dimension }
func (c CRS) Geographic() bool     { return c.geographic }
func (c CRS) InternalSRID() uint64 { return c.srid }
func (c CRS) Definition() CRSDefinition {
	d := c.definition
	d.Axes = append([]CRSAxis(nil), d.Axes...)
	return d
}

func invalidCRS(reason string) error {
	return provider.InvalidFeatureQueryError{Field: "crs", Reason: reason}
}

// ResolveCRS accepts only exact shipped public identifiers. It performs no I/O.
func ResolveCRS(uri string) (CRS, error) {
	if len(uri) == 0 || len(uri) > MaxCRSURIBytes {
		return CRS{}, invalidCRS("unrecognized identifier")
	}
	switch uri {
	case CRS84:
		return canonicalCRS(4326, 2, false, uri)
	case CRS84h:
		return canonicalCRS(4326, 3, false, uri)
	case "http://www.opengis.net/def/crs/EPSG/0/4326":
		return canonicalCRS(4326, 2, true, uri)
	case "http://www.opengis.net/def/crs/EPSG/0/4979":
		return canonicalCRS(4326, 3, true, uri)
	}
	// Fixed enumeration prevents alternative spellings and arbitrary EPSG inference.
	for _, srid := range canonicalSRIDs() {
		if srid != 4326 && uri == epsgURI(srid) {
			return canonicalCRS(srid, 2, false, uri)
		}
	}
	return CRS{}, invalidCRS("unrecognized identifier")
}

func epsgURI(srid uint64) string {
	return fmt.Sprintf("http://www.opengis.net/def/crs/EPSG/0/%d", srid)
}
func canonicalSRIDs() []uint64 {
	ids := make([]uint64, 0, 122)
	ids = append(ids, 4326, 3857)
	for zone := uint64(1); zone <= 60; zone++ {
		ids = append(ids, 32600+zone, 32700+zone)
	}
	return ids
}

func canonicalCRS(srid uint64, dimension int, swapped bool, uri string) (CRS, error) {
	p, err := crsconfig.NewHeightProjection(srid)
	if err != nil {
		return CRS{}, fmt.Errorf("features: canonical projection: %w", err)
	}
	axes := []CRSAxis{{"easting", "metre"}, {"northing", "metre"}}
	geographic := srid == 4326
	if geographic {
		axes = []CRSAxis{{"longitude", "degree"}, {"latitude", "degree"}}
	}
	if swapped {
		axes[0], axes[1] = axes[1], axes[0]
	}
	vertical := ""
	if dimension == 3 {
		axes = append(axes, CRSAxis{"ellipsoidal height", "metre"})
		vertical = CRS84h
	}
	d := CRSDefinition{URI: uri, Definition: p.Definition(), Datum: "WGS84", Axes: axes, Dimension: dimension, VerticalCRS: vertical}
	owned, err := crsconfig.NewFeatureProjection(p.Definition())
	if err != nil {
		return CRS{}, err
	}
	return CRS{definition: d, srid: srid, geographic: geographic, swapped: swapped, projection: owned}, nil
}

// NewApplicationCRS admits validated owned horizontal profiles. The caller must
// separately prove this is the source's effective definition. Custom profiles
// have no canonical authority and cannot establish height preservation.
func NewApplicationCRS(definition string, internalSRID uint64, dimension int) (CRS, error) {
	if len(definition) == 0 || len(definition) > MaxCRSDefinitionBytes || (dimension != 2 && dimension != 3) || internalSRID == 0 {
		return CRS{}, invalidCRS("invalid application definition")
	}
	owned, err := crsconfig.NewFeatureProjection(definition)
	if err != nil {
		return CRS{}, fmt.Errorf("features: application definition: %w", provider.ErrUnsupported)
	}
	var c CRS
	if owned.CanonicalSRID() == 0 {
		if dimension != 2 {
			return CRS{}, invalidCRS("custom definition does not prove a vertical datum")
		}
		c.definition = customHorizontalDescriptor(definition)
	} else {
		c, err = canonicalCRS(owned.CanonicalSRID(), dimension, false, "")
		if err != nil {
			return CRS{}, err
		}
	}
	c.projection = owned
	c.definition.Definition = definition
	c.srid = internalSRID
	digest := descriptorDigest(c.definition)
	c.definition.SHA256 = hex.EncodeToString(digest[:])
	uuid := digest[:16]
	uuid[6] = (uuid[6] & 0x0f) | 0x80
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	c.definition.URI = fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x", uuid[:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
	return c, nil
}

// The full admitted definition is retained in the descriptor digest. Datum and
// axis labels describe custom storage without inventing an EPSG/WGS84 identity.
func customHorizontalDescriptor(definition string) CRSDefinition {
	parameters := make(map[string]string)
	for _, word := range strings.Fields(definition) {
		key, value, _ := strings.Cut(strings.TrimPrefix(word, "+"), "=")
		parameters[key] = value
	}
	unit := parameters["units"]
	switch unit {
	case "", "m":
		unit = "metre"
	case "km":
		unit = "kilometre"
	}
	if factor := parameters["to_meter"]; factor != "" {
		unit = factor + " metre"
	}
	datum := parameters["datum"]
	if datum == "" {
		datum = "ellps=" + parameters["ellps"] + ";towgs84=" + parameters["towgs84"]
	}
	return CRSDefinition{Definition: definition, Datum: datum, Dimension: 2,
		Axes: []CRSAxis{{"easting", unit}, {"northing", unit}}}
}

func descriptorDigest(d CRSDefinition) [32]byte {
	h := sha256.New()
	for _, value := range append([]string{"tegola-feature-crs-v1", d.Definition, d.Datum, fmt.Sprint(d.Dimension), d.VerticalCRS}, axisValues(d.Axes)...) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		h.Write(size[:])
		h.Write([]byte(value))
	}
	var digest [32]byte
	copy(digest[:], h.Sum(nil))
	return digest
}
func axisValues(axes []CRSAxis) []string {
	values := make([]string, 0, len(axes)*2)
	for _, axis := range axes {
		values = append(values, axis.Name, axis.Unit)
	}
	return values
}

// ToInternalPosition converts wire axes to internal XY/Z without changing units.
func (c CRS) ToInternalPosition(position []float64) ([]float64, error) {
	return c.orderPosition(position)
}
func (c CRS) FromInternalPosition(position []float64) ([]float64, error) {
	return c.orderPosition(position)
}

// FromInternalXY reorders horizontal axes independently of height presence.
// Mixed geometry children retain their two coordinates; no Z is fabricated.
func (c CRS) FromInternalXY(position []float64) ([]float64, error) {
	if c.projection == nil || len(position) != 2 {
		return nil, invalidCRS("horizontal coordinates require a pair")
	}
	for _, value := range position {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, invalidCRS("coordinates must be finite")
		}
	}
	output := append([]float64(nil), position...)
	if c.swapped {
		output[0], output[1] = output[1], output[0]
	}
	return output, nil
}

// ToInternalXY is the inverse horizontal wire-axis permutation.
func (c CRS) ToInternalXY(position []float64) ([]float64, error) {
	return c.FromInternalXY(position)
}
func (c CRS) orderPosition(position []float64) ([]float64, error) {
	if c.projection == nil || len(position) != c.Dimension() {
		return nil, invalidCRS("coordinate dimension differs from CRS")
	}
	for _, value := range position {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, invalidCRS("coordinates must be finite")
		}
	}
	output := append([]float64(nil), position...)
	if c.swapped {
		output[0], output[1] = output[1], output[0]
	}
	return output, nil
}

// ForwardXY and InverseXY transform internal coordinate pairs, retaining no input.
func (c CRS) ForwardXY(xy []float64) ([]float64, error) {
	if c.projection == nil {
		return nil, invalidCRS("uninitialized CRS")
	}
	return c.projection.Forward(xy)
}
func (c CRS) InverseXY(xy []float64) ([]float64, error) {
	if c.projection == nil {
		return nil, invalidCRS("uninitialized CRS")
	}
	return c.projection.Inverse(xy)
}

// CollectionCRS is an immutable, instance-local catalog. Zero is unavailable.
type CollectionCRS struct {
	descriptors            map[string]CRS
	uris                   []string
	defaultURI, storageURI string
	spatial                provider.SpatialMetadata
}

func (c CollectionCRS) URIs() []string     { return append([]string(nil), c.uris...) }
func (c CollectionCRS) DefaultURI() string { return c.defaultURI }
func (c CollectionCRS) StorageURI() string { return c.storageURI }
func (c CollectionCRS) Resolve(uri string) (CRS, error) {
	if len(uri) == 0 || len(uri) > MaxCRSURIBytes {
		return CRS{}, invalidCRS("unrecognized identifier")
	}
	if c.descriptors == nil {
		return CRS{}, fmt.Errorf("features: collection CRS unavailable: %w", provider.ErrUnsupported)
	}
	result, ok := c.descriptors[uri]
	if !ok {
		return CRS{}, invalidCRS("identifier is not supported by collection")
	}
	return result, nil
}
func (c CollectionCRS) ValidateOutput(uri string) (CRS, error) {
	if uri == "" {
		uri = c.defaultURI
	}
	crs, err := c.Resolve(uri)
	if err != nil {
		return CRS{}, err
	}
	if (c.spatial.Dimension == provider.DimensionXY) != (crs.Dimension() == 2) {
		return CRS{}, invalidCRS("representation cannot lose or invent height")
	}
	return crs, nil
}
func (c CollectionCRS) ValidateBounds(uri string, coordinateCount int) (CRS, error) {
	if uri == "" {
		if c.descriptors == nil {
			return CRS{}, fmt.Errorf("features: collection CRS unavailable: %w", provider.ErrUnsupported)
		}
		if coordinateCount == 6 {
			return ResolveCRS(CRS84h)
		} else if coordinateCount == 4 {
			return ResolveCRS(CRS84)
		} else {
			return CRS{}, invalidCRS("bbox requires four or six coordinates")
		}
	}
	crs, err := c.Resolve(uri)
	if err != nil {
		return CRS{}, err
	}
	if coordinateCount != 2*crs.Dimension() {
		return CRS{}, invalidCRS("bbox dimension differs from CRS")
	}
	return crs, nil
}

// NewCollectionCRS requires already proven source identity. Mixed storage has no
// uniform-dimensional storage URI; its default uses unknown, never invented Z.
func NewCollectionCRS(storage CRS, spatial provider.SpatialMetadata) (CollectionCRS, error) {
	if err := spatial.Validate(); err != nil {
		return CollectionCRS{}, err
	}
	if storage.projection == nil || strings.TrimSpace(storage.URI()) == "" {
		return CollectionCRS{}, invalidCRS("source CRS is uninitialized")
	}
	dimension := 2
	if spatial.Dimension != provider.DimensionXY {
		dimension = 3
	}
	if storage.Dimension() != dimension {
		return CollectionCRS{}, invalidCRS("source CRS dimension differs from spatial profile")
	}
	catalog := CollectionCRS{descriptors: make(map[string]CRS), spatial: spatial, defaultURI: CRS84, storageURI: storage.URI()}
	if dimension == 3 {
		catalog.defaultURI = CRS84h
	}
	if spatial.Dimension == provider.DimensionMixedXYXYZ {
		catalog.storageURI = ""
	}
	add := func(c CRS) error {
		if old, ok := catalog.descriptors[c.URI()]; ok {
			if descriptorDigest(old.definition) != descriptorDigest(c.definition) {
				return invalidCRS("CRS identity collision")
			}
			return nil
		}
		if len(catalog.descriptors) >= MaxCollectionCRSs {
			return invalidCRS("too many collection CRSs")
		}
		catalog.descriptors[c.URI()] = c
		return nil
	}
	for _, uri := range []string{catalog.defaultURI} {
		c, err := ResolveCRS(uri)
		if err != nil {
			return CollectionCRS{}, err
		}
		if err = add(c); err != nil {
			return CollectionCRS{}, err
		}
	}
	targets := []uint64{4326, 3857}
	if sourceProfile := storage.projection.CanonicalSRID(); sourceProfile != 0 && sourceProfile != 4326 && sourceProfile != 3857 {
		targets = append(targets, sourceProfile)
	}
	for _, srid := range targets {
		var c CRS
		var err error
		if dimension == 2 {
			c, err = canonicalCRS(srid, 2, srid == 4326, epsgURI(srid))
		} else if srid == 4326 {
			c, err = ResolveCRS("http://www.opengis.net/def/crs/EPSG/0/4979")
		} else {
			p, e := crsconfig.NewHeightProjection(srid)
			if e != nil {
				return CollectionCRS{}, e
			}
			c, err = NewApplicationCRS(p.Definition(), srid, 3)
		}
		if err != nil {
			return CollectionCRS{}, err
		}
		if err = add(c); err != nil {
			return CollectionCRS{}, err
		}
	}
	if err := add(storage); err != nil {
		return CollectionCRS{}, err
	}
	catalog.uris = []string{catalog.defaultURI}
	others := make([]string, 0, len(catalog.descriptors)-1)
	for uri := range catalog.descriptors {
		if uri != catalog.defaultURI {
			others = append(others, uri)
		}
	}
	sort.Strings(others)
	catalog.uris = append(catalog.uris, others...)
	return catalog, nil
}
