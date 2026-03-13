package session

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"ekso/internal/auth"
	"ekso/internal/inventory"
	"ekso/internal/procedure"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type HostClient struct {
	Item   inventory.InventoryItem
	Client *ssh.Client
}

type TaskResult struct {
	HostID string
	ProcID string
	TaskID string
	Output string
	Err    error
}

func DialSSHToHost(host inventory.Host, auth auth.Auth, timeout int64) (*ssh.Client, error) {
	khPath := filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")
	hostKeyCallback, err := knownhosts.New(khPath)
	if err != nil {
		return nil, fmt.Errorf("knownhosts: %w", err)
	}

	configAuthMethod, err := auth.DeriveAuthMethod()
	if err != nil {
		return nil, err
	}

	cfg := &ssh.ClientConfig{
		User: host.User,
		Auth: []ssh.AuthMethod{
			configAuthMethod,
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         time.Duration(timeout) * time.Second,
	}
	addr := net.JoinHostPort(host.Address, strconv.FormatUint(host.Port, 10))
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	return client, nil
}

func RunCommand(client *ssh.Client, cmd string, shell string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer func() {
		err = sess.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "close session: %v\n", err)
		}
	}()

	// TODO wrap this in a more robust check so we can determine shells.
	// Maybe make it a CLI flag
	wrappedCmd := fmt.Sprintf(`%q -lc %q`, shell, cmd)

	stdout, err := sess.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := sess.Start(wrappedCmd); err != nil {
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

func CloseClients(clients []HostClient) error {
	for _, hc := range clients {
		if hc.Client != nil {
			err := hc.Client.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func RunTaskOnHostWithoutBarrier(hc HostClient, procs []procedure.Procedure, resultsChannel chan TaskResult, wg *sync.WaitGroup) error {
	defer wg.Done()
	for _, proc := range procs {
		for _, task := range proc.Tasks {
			var out string
			var err error
			shell := task.Shell
			switch {
			case len(task.Command.ArgV) > 0:
				cmd := procedure.ArgVToShell(task.Command.ArgV)
				out, err = RunCommand(hc.Client, cmd, shell)
			case task.Command.Exec != "":
				out, err = RunCommand(hc.Client, task.Command.Exec, shell)
			default:
				err = fmt.Errorf("no command field specified")
			}
			resultsChannel <- TaskResult{hc.Item.ID, proc.ID, task.ID, out, err}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func RunTaskOnHost(hc HostClient, proc procedure.Procedure, task procedure.Task, resultsChannel chan TaskResult, wg *sync.WaitGroup) error {
	defer wg.Done()
	var out string
	var err error
	shell := task.Shell
	switch {
	case len(task.Command.ArgV) > 0:
		cmd := procedure.ArgVToShell(task.Command.ArgV)
		out, err = RunCommand(hc.Client, cmd, shell)
	case task.Command.Exec != "":
		out, err = RunCommand(hc.Client, task.Command.Exec, shell)
	default:
		err = fmt.Errorf("no command field specified")
	}

	resultsChannel <- TaskResult{hc.Item.ID, proc.ID, task.ID, out, err}

	return nil
}
