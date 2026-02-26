package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/go-yaml/yaml"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var EksoCLI struct {
	File string `short:"f" help:"path to the yaml file defining procedures"`
	DryRun bool `short:"d" help:"only output info without running any tasks" default:"false"` 
}


type Procedure struct {
	ID string `yaml:"id"`
	Tasks []Task
}

type Task struct {
	ID string `yaml:"id"`
	Command Command `yaml:"command"`
	Critical bool `yaml:"critical,omitempty"`
}

type Command struct {
	ArgV *[]string `yaml:"argv,omitempty"`
	Exec *string `yaml:"exec,omitempty"`
}


type Config struct {
	Inventory []InventoryItem `yaml:"inventory"`
	Procedures []Procedure `yaml:"procedures"`
}

type Host struct {
	Address string `yaml:"address"`
	Port uint64 `yaml:"port,omitempty"`
	User string `yaml:"user,omitempty"`
}

type InventoryItem struct  {
	ID string `yaml:"id"`
	Host Host `yaml:"host"`
	Auth Auth `yaml:"auth"`
}

type Auth struct {
	Password *PasswordAuth`yaml:"password"`
	Key *KeyAuth `yaml:"key,omitempty"`
}

type PasswordAuth struct {
	Env string `yaml:"env,omitempty"`
}

func (a Auth) DeriveAuthMethod() (ssh.AuthMethod, error) {
	switch {
	case a.Password != nil:
		p := a.Password
		if p.Env == ""{
			return nil, fmt.Errorf("password auth requires env variable")	
		}
		pass := os.Getenv(p.Env)
		if pass == "" {
			return nil, fmt.Errorf("environment variable %s not set", p.Env)
		}
		return ssh.Password(pass), nil
	case a.Key != nil:
		// Key auth example
		k := a.Key
		if k.Path == "" {
			return nil, fmt.Errorf("key auth requires path")
		}

		keyBytes, err := os.ReadFile(k.Path)
		if err != nil {
			return nil, err
		}

		var signer ssh.Signer

		// If passphrase provided
		if k.PassphraseEnv != "" {
			passphrase := os.Getenv(k.PassphraseEnv)
			if passphrase == "" {
				return nil, fmt.Errorf("env %s not set", k.PassphraseEnv)
			}

			signer, err = ssh.ParsePrivateKeyWithPassphrase(
				keyBytes,
				[]byte(passphrase),
			)
		} else {
			signer, err = ssh.ParsePrivateKey(keyBytes)
		}

		if err != nil {
			return nil, err
		}

		return ssh.PublicKeys(signer), nil
	default:
		return nil, fmt.Errorf("auth: no valid auth method provided")
	}
}

type KeyAuth struct {
	Path          string `yaml:"path,omitempty"`
	PassphraseEnv string `yaml:"passphrase_env,omitempty"`
}

func dialSSHToHost(host Host, authMethod Auth) (*ssh.Client, error) {
	khPath := filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")
	hostKeyCallback, err := knownhosts.New(khPath)

	if err != nil {
		return nil, fmt.Errorf("knownhosts: %w", err)
	}

	configAuthMethod, err := authMethod.DeriveAuthMethod()
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User: host.User,
		Auth: []ssh.AuthMethod{
			configAuthMethod,
		},
		HostKeyCallback: hostKeyCallback,
		Timeout: 10*time.Second,
	}
	port := host.Port
	if port == 0 {
		port = 22
	}
	addr := net.JoinHostPort(host.Address,strconv.FormatUint(port, 10))
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	return client, nil
}

func shellEscape(s string) string {
	// Wrap in single quotes and escape any embedded single quotes:
	// abc'd -> 'abc'"'"'d'
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func argvToShell(argv []string) string {
	escaped := make([]string, 0, len(argv))
	for _, a := range argv {
		escaped = append(escaped, shellEscape(a))
	}
	return strings.Join(escaped, " ")
}

func runCommand(client *ssh.Client, cmd string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	stdout, err := sess.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := sess.Start(cmd); err != nil {
		return "", fmt.Errorf("start: %w", err)
	}

	outBytes, _ := io.ReadAll(stdout)
	errBytes, _ := io.ReadAll(stderr)

	if err := sess.Wait(); err != nil {
		// include stderr in error message
		return "", fmt.Errorf("wait: %w\nstderr: %s", err, string(errBytes))
	}

	return string(outBytes), nil
}

func main() {
	ctx := kong.Parse(&EksoCLI)
  	_ = ctx

	configData, err := os.ReadFile(EksoCLI.File)
	if err != nil {
		panic(fmt.Errorf("failed to read conffig file: %w", err))
	}

	var config Config
	if err := yaml.Unmarshal(configData, &config); err != nil {
		panic(fmt.Errorf("failed to unmarshal yaml: %w", err))
	}

	for _, host := range config.Inventory {
		client, err := dialSSHToHost(host.Host, host.Auth)
		if err != nil {
			panic(err)
		}
		defer client.Close()
		// Run each procedure
		for _, proc := range config.Procedures {
			for _, task := range proc.Tasks {
				// Likely need to figure out how to manage this as well.
				// We should switch on the task command
				switch {
				case task.Command.ArgV != nil:
					cmd := argvToShell(*task.Command.ArgV)
					out, err := runCommand(client, cmd)
					_ = out
					if err != nil {
						panic(err)
					}
				case task.Command.Exec != nil:
					out, err := runCommand(client, *task.Command.Exec)
					_ = out
					if err != nil {
						panic(err)
					}
				default:
					panic(fmt.Errorf("command: unknown command type %v", task.Command))
				}
			} 
		}
	}
}
