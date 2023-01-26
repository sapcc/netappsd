package netappsd

import (
	"encoding/json"
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

type Filer struct {
	Name string `json:"name" yaml:"name"`
	Host string `json:"host" yaml:"host"`
	AZ   string `json:"availability_zone" yaml:"availability_zone"`
	IP   string `json:"ip,omitempty" yaml:"ip,omitempty"`
}

type Filers map[string]Filer

type FilerConfig struct {
	filers     Filers
	configPath string
	nc         *Netbox
}

func NewFilerConfig(p string, nc *Netbox) (*FilerConfig, error) {
	c := &FilerConfig{
		configPath: p,
		nc:         nc,
	}
	err := c.LoadConfig()
	return c, err
}

func (c *FilerConfig) LoadConfig() error {
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, &c.filers)
}

func (c *FilerConfig) Fetch(region, query string) (old Filers, neo Filers, err error) {
	old = c.filers
	// query new filers from NetBox
	neo, err = GetFilers(c.nc, region, query)
	if err != nil {
		return nil, nil, err
	}
	c.filers = neo
	return old, neo, nil
}

// write to config file
func (c *FilerConfig) Write() error {
	// s, err := yaml.Marshal(c.filers.list())
	s, err := yaml.Marshal(c.filers)
	if err != nil {
		return err
	}
	return os.WriteFile(c.configPath, []byte(s), os.FileMode(0644))
}

func (f Filers) list() []Filer {
	lf := make([]Filer, 0)
	for _, ff := range f {
		lf = append(lf, ff)
	}
	return lf
}

func (f Filers) JsonString() string {
	s, err := json.Marshal(f.list())
	if err != nil {
		log.Fatal(err)
	}
	return string(s)
}
