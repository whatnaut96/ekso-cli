package auth

import (
	"fmt"

	"golang.org/x/crypto/ssh"
)

type Auth struct {
	Password *PasswordAuth `yaml:"password,omitempty"`
	Key      *KeyAuth      `yaml:"key,omitempty"`
}

func (a Auth) String() string {
	switch {
	case a.Password != nil:
		return fmt.Sprintf("password(env=%s)", a.Password.Env)
	case a.Key != nil:
		if a.Key.PassphraseEnv == "" {
			return fmt.Sprintf("key(path=%s)", a.Key.Path)
		} else {
			return fmt.Sprintf("key(path=%s, passphrase_env=%s)", a.Key.Path, a.Key.PassphraseEnv)
		}
	default:
		return "auth(<none>)"
	}
}

func (a Auth) DeriveAuthMethod() (ssh.AuthMethod, error) {
	switch {
	case a.Password != nil:
		return a.Password.getAuthMethod()
	case a.Key != nil:
		return a.Key.getAuthMethod()
	default:
		return nil, fmt.Errorf("auth: no valid auth method provided")
	}
}
