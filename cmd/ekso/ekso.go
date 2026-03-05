// TODO: Follow same filter pattern for procedure filtering
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"ekso/internal/inventory"
	"ekso/internal/procedure"
	"ekso/internal/session"

	"github.com/go-yaml/yaml"
	"github.com/urfave/cli/v3"
)

type Config struct {
	Inventory  []inventory.InventoryItem `yaml:"inventory"`
	Procedures []procedure.Procedure     `yaml:"procedures"`
}

const (
	DefaultSSHTimeout       = 10
	ResultChannelBufferSize = 64
)

func main() {
	cmd := &cli.Command{
		UseShortOptionHandling: true,
		Name:                   "ekso",
		Usage:                  "an distributed task runner to run on any host",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "path to the yaml file defining inventory and procedures"},
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "verbose output", Value: false},
			&cli.BoolFlag{Name: "dry-run", Aliases: []string{"d"}, Usage: "only output info without running any tasks", Value: false},
			&cli.BoolFlag{Name: "no-barrier", Usage: "execute tasks without a barrier strategy", Value: false},
			&cli.Int64Flag{Name: "timeout", Usage: "timeout length for ssh connections", Value: DefaultSSHTimeout},
			&cli.StringFlag{Name: "procedure", Aliases: []string{"p"}, Usage: "tag name of the procedure to run"},
			&cli.StringFlag{Name: "inventory", Aliases: []string{"i"}, Usage: "tag name of the hosts to run tasks on"},
			&cli.BoolFlag{Name: "all-hosts", Usage: "run procedure on all hosts", Value: false},
			&cli.BoolFlag{Name: "all-procedures", Usage: "run all procedures on the host", Value: false},
		},

		Action: func(ctx context.Context, cmd *cli.Command) error {
			var configData []byte
			var err error

			if cmd.String("file") != "" {
				configData, err = os.ReadFile(cmd.String("file"))
			} else if cmd.Bool("stdin") {
				// Read from stdin
				configData, err = io.ReadAll(os.Stdin)
			} else {
				return fmt.Errorf("no input method specified")
			}

			if err != nil {
				panic(fmt.Errorf("failed to read config input: %w", err))
			}

			var config Config
			if err := yaml.Unmarshal(configData, &config); err != nil {
				panic(fmt.Errorf("failed to unmarshal yaml: %w", err))
			}

			clients := make([]session.HostClient, 0, len(config.Inventory))
			for _, item := range config.Inventory {
				useHost := cmd.Bool("all-hosts") || item.ID == cmd.String("inventory")
				if !useHost {
					continue
				}

				client, err := session.DialSSHToHost(item.Host, item.Auth, cmd.Int64("timeout"))
				if err != nil {
					return fmt.Errorf("dial ssh host=%s: %w", item.ID, err)
				}

				clients = append(clients, session.HostClient{
					Item:   item,
					Client: client,
				})
			}

			defer func() {
				if err := session.CloseClients(clients); err != nil {
					fmt.Fprintf(os.Stderr, "close session: %v\n", err)
				}
			}()

			procs := make([]procedure.Procedure, 0, len(config.Procedures))
			for _, proc := range config.Procedures {
				useProcedure := cmd.Bool("all-procedures") || proc.ID == cmd.String("procedure")
				if !useProcedure {
					continue
				}
				procs = append(procs, proc)
			}

			_ = procs

			if cmd.Bool("no-barrier") {
				resultsCh := make(chan session.TaskResult, ResultChannelBufferSize)
				var wg sync.WaitGroup
				wg.Add(len(clients))

				for _, hc := range clients {
					go func() {
						if err := session.RunTaskOnHostWithoutBarrier(hc, procs, resultsCh, &wg); err != nil {
							resultsCh <- session.TaskResult{HostID: hc.Item.ID, ProcID: "", TaskID: "", Err: err}
						}
					}()
					go func() { wg.Wait(); close(resultsCh) }()
					var failures int
					for r := range resultsCh {
						if r.Err != nil {
							failures++
							fmt.Printf("[FAIL] host=%s proc=%s task=%s err=%v\n", r.HostID, r.ProcID, r.TaskID, r.Err)
						} else {
							fmt.Printf("[OK]  host=%s proc=%s task=%s\n", r.HostID, r.ProcID, r.TaskID)
						}
					}
					if failures > 0 {
						return fmt.Errorf("execution failed: failures=%d", failures)
					}
				}
			} else {
				for _, proc := range procs {
					for _, task := range proc.Tasks {
						resultsCh := make(chan session.TaskResult, ResultChannelBufferSize)
						var wg sync.WaitGroup
						wg.Add(len(clients))
						for _, hc := range clients {
							go func() {
								if err := session.RunTaskOnHost(hc, proc, task, resultsCh, &wg); err != nil {
									resultsCh <- session.TaskResult{
										HostID: hc.Item.ID, // adjust field names
										ProcID: "",         // or "all"/"bootstrap"
										TaskID: "",
										Err:    err,
									}
								}

							}()
						}
						go func() { wg.Wait(); close(resultsCh) }()
						var failures int
						for r := range resultsCh {
							if r.Err != nil {
								failures++
								fmt.Printf("[FAIL] host=%s proc=%s task=%s err=%v\n", r.HostID, r.ProcID, r.TaskID, r.Err)
							} else {
								fmt.Printf("[OK]  host=%s proc=%s task=%s\n", r.HostID, r.ProcID, r.TaskID)
							}
						}
						if failures > 0 {
							return fmt.Errorf("task failed: proc=%s task=%s failures=%d", proc.ID, task.ID, failures)
						}
					}
				}
			}
			return nil
		},
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
