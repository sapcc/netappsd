package netappsd

import (
	"encoding/json"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

type Filer struct {
	Name             string `json:"name" yaml:"name"`
	Host             string `json:"host" yaml:"host"`
	AvailabilityZone string `json:"availability_zone" yaml:"availability_zone"`
	IP               string `json:"ip,omitempty" yaml:"ip,omitempty"`
}

type Filers []Filer

type FilerConfig struct {
	filers     Filers
	configFile string
	templates  map[string]*template.Template
	nc         *Netbox
}

func NewFilerConfig(cfgdir string, nc *Netbox) (*FilerConfig, error) {
	var err error
	c := &FilerConfig{
		nc:         nc,
		configFile: filepath.Join(cfgdir, "filers.yaml"),
		templates:  make(map[string]*template.Template),
	}
	tfiles, err := filepath.Glob(filepath.Join(cfgdir, "*.yaml.tpl"))
	if err != nil {
		return nil, err
	}
	for _, tf := range tfiles {
		t, err := template.ParseFiles(tf)
		if err != nil {
			return nil, err
		}
		c.templates[strings.TrimSuffix(tf, ".tpl")] = t
	}
	return c, err
}

func (c *FilerConfig) LoadConfig() error {
	data, err := os.ReadFile(c.configFile)
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
func (c *FilerConfig) RenderAndWrite() error {
	s, err := yaml.Marshal(c.filers)
	if err != nil {
		return err
	}
	err = os.WriteFile(c.configFile, []byte(s), os.FileMode(0644))
	if err != nil {
		return err
	}
	for fn, t := range c.templates {
		f, err := os.Create(fn)
		if err != nil {
			return err
		}
		err = t.Execute(f, c.filers)
		if err != nil {
			return err
		}
	}
	return nil
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
