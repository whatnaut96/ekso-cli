# ekso
ekso is a deterministic, remote/distributed procedure runner over SSH

It allows you to define ordered procedures composed of tasks and execute them across multiple hosts using explicit concurrency strategies.

This is not a configuration management tool that handles or converges state. It simply executes procedures predictably.

## Installation
### Build from source:
```bash
go build cmd/ekso/ekso.go

```
## Usage
***ekso is still pre 1.0, breaking changes are to be expected.***
```
ekso -f <config.yaml> -p <procedure> -i <inventory> 

```
### Flags:
```bash
--file string, -f string       path to the yaml file defining inventory and procedures
--verbose, -v                  verbose output
--dry-run, -d                  only output info without running any tasks
--no-barrier                   execute tasks without a barrier strategy
--timeout int                  timeout length for ssh connections (default: 10)
--procedure string, -p string  tag name of the procedure to run
--inventory string, -i string  tag name of the hosts to run tasks on
--all-hosts                    run procedure on all hosts
--all-procedures               run all procedures on the host
--help, -h                     show help

```
## Concurrency Strategies
### Barrier (default)
All host execut task N.
ekso waits for all hosts to complete.
Then task N+1 begins.

Deterministic, lock-step execution.

### Independent (--no-barrier)
Each host executes the full procedure independently.
No cross-host synchronization between tasks.

## Configuration example
```yaml
inventory:
  - id: local 
    host:
      address: localhost
      user: me
    auth:
      password:
        env: LOCALPASS
procedures:
- id: default
  tasks:
    - id: touch_file
      shell: "zsh"
      command:
        argv: ["touch", "/some/path/to/test.txt"]
    - id: brew_update
      shell: "zsh"
      command:
        exec: "brew update"
```

## Design Goals
ekso is not a:
- Configuration management tool
- State reconciler
- Orchestrator/Scheduler
- IaC substiture

It's a procedural execution engine over SSH with the goal of being exlpicit, deterministic, and relatively lightweight.

