package main

import (
	"encoding/json"
	"log"

	"gopkg.in/yaml.v2"
)

type Filer struct {
	Name string `json:"name" yaml:"name"`
	Host string `json:"host" yaml:"host"`
	AZ   string `json:"az" yaml:"az"`
}

type Filers map[string]Filer

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

func (f Filers) YamlString() string {
	s, err := yaml.Marshal(f.list())
	if err != nil {
		log.Fatal(err)
	}
	return string(s)
}
