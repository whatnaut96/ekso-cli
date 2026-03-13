package procedure

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const MinToolCount = 2

type Procedure struct {
	ID    string `yaml:"id"`
	Tasks []Task
}

type Task struct {
	ID      string  `yaml:"id"`
	Command Command `yaml:"command"`
	Shell   string  `yaml:"shell"`
}

type Command struct {
	ArgV    []string           `yaml:"argv,omitempty"`
	Exec    string             `yaml:"exec,omitempty"`
	With    *yaml.Node         `yaml:"with,omitempty"`
}

func (c *Command) UnmarshalYAML(value *yaml.Node) error {
	var tmp struct {
		ArgV       []string   `yaml:"argv,omitempty"`
		Exec       string     `yaml:"exec,omitempty"`
	}

	if err := value.Decode(&tmp); err != nil {
		return err
	}

	hasArgv := len(tmp.ArgV) > 0
	hasExec := tmp.Exec != ""

	if hasExec == hasArgv {
		return fmt.Errorf("command must contain exactly one of: argv, exec")
	}
	if hasArgv {
		for i, s := range tmp.ArgV {
			if s == "" {
				return fmt.Errorf("command.argv[%d] must be non-empty", i)
			}
		}
	}

	*c = Command{
		ArgV:    tmp.ArgV,
		Exec:    tmp.Exec,
	}
	return nil
}


func ArgVToShell(argv []string) string {
	escaped := make([]string, 0, len(argv))
	for _, a := range argv {
		// Escaping the shell stuff for argv to avoid any issues
		escaped = append(escaped, "'"+strings.ReplaceAll(a, "'", `'"'"'`)+"'")
	}
	return strings.Join(escaped, " ")
}
