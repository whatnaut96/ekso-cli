package auth

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

type PasswordAuth struct {
	Env string `yaml:"env,omitempty"`
}

func (p PasswordAuth) getAuthMethod() (ssh.AuthMethod, error) {
	if p.Env == "" {
		return nil, fmt.Errorf("password auth requires env variables")
	}
	pass := os.Getenv(p.Env)
	if pass == "" {
		return nil, fmt.Errorf("environment variable %s not set", p.Env)
	}
	return ssh.Password(pass), nil
}
