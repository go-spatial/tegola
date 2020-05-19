package mvtprovider

// Layer holds information about a query.
type Layer struct {
	// Name is the name of the Layer as recognized by the provider
	Name string
	// MVTName is the name of the layer to encode into the MVT.
	// this is often used when different provider layers are used
	// at different zoom levels but the MVT layer name is consistent
	MVTName string
}
