package auth

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

type KeyAuth struct {
	Path          string `yaml:"path,omitempty"`
	PassphraseEnv string `yaml:"passphrase_env,omitempty"`
}

func (k KeyAuth) getAuthMethod() (ssh.AuthMethod, error) {
	if k.Path == "" {
		return nil, fmt.Errorf("key auth requires path")
	}
	keyBytes, err := os.ReadFile(k.Path)
	if err != nil {
		return nil, err
	}

	var signer ssh.Signer
	var sshKeyErr error
	if k.PassphraseEnv != "" {
		passphrase := os.Getenv(k.PassphraseEnv)
		if passphrase == "" {
			sshKeyErr = errors.New(fmt.Sprintf("env %s not set", k.PassphraseEnv))
		}
		signer, sshKeyErr = ssh.ParsePrivateKeyWithPassphrase(
			keyBytes,
			[]byte(passphrase),
		)
	} else {
		signer, sshKeyErr = ssh.ParsePrivateKey(keyBytes)
	}
	if sshKeyErr != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}
