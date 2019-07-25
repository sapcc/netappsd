package netappsd

import (
	"encoding/json"
	"log"

	"gopkg.in/yaml.v2"
)

type Filer struct {
	Name string `json:"name" yaml:"name"`
	Host string `json:"host" yaml:"host"`
}

type Filers []Filer

func (f Filers) JsonString() string {
	s, err := json.Marshal(f)
	if err != nil {
		log.Fatal(err)
	}
	return string(s)
}

func (f Filers) YamlString() string {
	s, err := yaml.Marshal(f)
	if err != nil {
		log.Fatal(err)
	}
	return string(s)
}
