package mbe

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Param struct {
	ID string `yaml:"id"`
	Page uint8 `yaml:"page"`
	Addr []uint8 `yaml:"addr"`
	Name string `yaml:"name"`
	Desc string `yaml:"desc"`
	Scale Scale `yaml:"scale"`
	Bits map[uint16]string `yaml:"bits"`
}

type Scale struct {
	Units string `yaml:"units"`
	ScaleMin float32 `yaml:"scale_min"`
	ScaleMax float32 `yaml:"scale_max"`
	DisplayMin float32 `yaml:"display_min"`
	DisplayMax float32 `yaml:"display_max"`
	Precision int `yaml:"precision"`
}

func LoadParams(f string) ([]Param, error) {
	b, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}
	m := []Param{}
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}
