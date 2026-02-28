package procedure

// TODO: Validate the builtins with a set that contains the valid builtins

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type Procedure struct {
	ID    string `yaml:"id"`
	Tasks []Task
}

type Task struct {
	ID      string  `yaml:"id"`
	Command Command `yaml:"command"`
	Shell string `yaml:"shell"`
}

type Command struct {
	ArgV    []string   `yaml:"argv,omitempty"`
	Exec    string     `yaml:"exec,omitempty"`
	Builtin *BuiltinInvocation     `yaml:"builtin,omitempty"`
	With    *yaml.Node `yaml:"with,omitempty"`
}

type BuiltinInvocation struct {
	Tool string
	Action string
	Args *yaml.Node
}

func (c *Command) UnmarshalYAML(value *yaml.Node) error {
	var tmp struct {
		ArgV       []string  `yaml:"argv,omitempty"`
		Exec       string    `yaml:"exec,omitempty"`
		BuiltinRaw *yaml.Node `yaml:"builtin,omitempty"`
		With       *yaml.Node `yaml:"with,omitempty"`
	}

	if err := value.Decode(&tmp); err != nil {
		return err
	}

	hasArgv := len(tmp.ArgV) > 0
	hasExec := tmp.Exec != ""
	hasBuiltin := tmp.BuiltinRaw != nil && tmp.BuiltinRaw.Kind != 0

	count := 0
	if hasArgv {
		count++
	}
	if hasExec {
		count++
	}
	if hasBuiltin {
		count++
	}
	if count != 1 {
		return fmt.Errorf("command must contain exactly one of: argv, exec, builtin")
	}

	if hasArgv {
		for i, s := range tmp.ArgV {
			if s == "" {
				return fmt.Errorf("command.argv[%d] must be non-empty", i)
			}
		}
		if tmp.With != nil {
			return fmt.Errorf("command.with is only valid with builtin commands")
		}
	}
	if hasExec {
		if tmp.With != nil {
			return fmt.Errorf("command.with is only valid with builtin commands")
		}
	}
	var builtin *BuiltinInvocation
	if hasBuiltin {
		tool, action, args, err := parseBuiltin(tmp.BuiltinRaw)
		if err != nil {
			return err
		}
		builtin = &BuiltinInvocation{
			Tool:   tool,
			Action: action,
			Args:   args,
		}
		// with is allowed for builtin (optional)
	}	

	*c = Command{
		ArgV:    tmp.ArgV,
		Exec:    tmp.Exec,
		Builtin: builtin,
		With:    tmp.With,
	}
	return nil
}

// parseBuiltin expects a mapping with exactly one key (tool),
// whose value is a mapping with exactly one key (action),
// whose value is the args node.
func parseBuiltin(n *yaml.Node) (tool string, action string, args *yaml.Node, err error) {
	if n.Kind != yaml.MappingNode {
		return "", "", nil, fmt.Errorf("command.builtin must be a mapping (e.g. builtin: {helm: {install: [...]}})")
	}
	if len(n.Content) != 2 {
		return "", "", nil, fmt.Errorf("command.builtin must contain exactly one builtin tool (got %d keys)", len(n.Content)/2)
	}

	toolKey := n.Content[0]
	toolVal := n.Content[1]
	if toolKey.Kind != yaml.ScalarNode || toolKey.Value == "" {
		return "", "", nil, fmt.Errorf("command.builtin tool name must be a non-empty scalar")
	}
	tool = toolKey.Value

	if toolVal.Kind != yaml.MappingNode {
		return "", "", nil, fmt.Errorf("command.builtin.%s must be a mapping of actions", tool)
	}
	if len(toolVal.Content) != 2 {
		return "", "", nil, fmt.Errorf("command.builtin.%s must contain exactly one action (got %d keys)", tool, len(toolVal.Content)/2)
	}

	actionKey := toolVal.Content[0]
	actionVal := toolVal.Content[1]
	if actionKey.Kind != yaml.ScalarNode || actionKey.Value == "" {
		return "", "", nil, fmt.Errorf("command.builtin.%s action name must be a non-empty scalar", tool)
	}
	action = actionKey.Value

	// args can be sequence or mapping depending on the builtin;
	// your example uses a sequence.
	args = actionVal
	return tool, action, args, nil
}

func ArgVToShell(argv []string) string {
	escaped := make([]string, 0, len(argv))
	for _, a := range argv {
		// Escaping the shell stuff for argv to avoid any issues
		escaped = append(escaped, "'"+strings.ReplaceAll(a, "'", `'"'"'`)+"'")
	}
	return strings.Join(escaped, " ")
}
