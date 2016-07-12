package server

type Config struct {
	Providers map[string]Provider
	Maps      map[string]Map
	Layers    map[string]Layer
}

type Provider struct {
	Name     string
	Type     string
	Host     string
	Port     uint16
	Database string
	User     string
	Password string
}

type Map struct {
	Name  string
	Layer string
}

type Layer struct {
	Name     string
	Provider string
	SQL      string
}

//	holds the instantiated config
var conf Config

//	look up a map
func (c *Config) Map(name string) {

}

//	fetch layer
func (c *Config) Layer(name string) {

}
