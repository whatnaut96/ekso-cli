package inventory

import (
	"os"

	"ekso/internal/auth"
)

type InventoryItem struct {
	ID   string    `yaml:"id"`
	Host Host      `yaml:"host"`
	Auth auth.Auth `yaml:"auth"`
}

type Host struct {
	Address string `yaml:"address"`
	Port    uint64 `yaml:"port,omitempty"`
	User    string `yaml:"user,omitempty"`
}

func (h *Host) UnmarshalYAML(unmarshal func(any) error) error {
	type raw Host
	var r raw
	if err := unmarshal(&r); err != nil {
		return err
	}
	if r.Port == 0 {
		r.Port = 22
	}

	if r.User == "" {
		r.User = os.Getenv("USER")
		if r.User == "" {
			panic("no user provided and $USER environment variable not found for default")
		}
	}

	*h = Host(r)
	return nil
}
