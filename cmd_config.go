package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/urfave/cli/v3"
)

func cmdConfig() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "manage configuration file",
		Commands: []*cli.Command{
			cmdConfigShow(),
			cmdConfigWrite(),
			cmdConfigRemove(),
		},
	}
}

var (
	DESCRIPTION_CONFIG_SHOW = `Show gdocker configuration.
	This command reads the gdocker configuration file and displays its contents.

	Examples)
	#> gdocker config show`
)

func cmdConfigShow() *cli.Command {
	return &cli.Command{
		Name:               "show",
		Usage:              "show gdocker configuration",
		UsageText:          ``,
		CustomHelpTemplate: TMPL_SUBCOMMAND_HELP,
		ArgsUsage:          "[options]",
		Description:        DESCRIPTION_CONFIG_SHOW,
		Before:             setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_CONFIG_DEFAULT,
			FLAG_SHOW_ABSPATH,
			FLAG_VERBOSE,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("config show", getLogLevel(cmd.Int("verbose")))
			slog.SetDefault(logger)

			c, file := loadConfig(cmd)
			fmt.Printf(`config file           : '%s'
Docker binary         : '%s'
Docker image directory: '%s'
Show absolute path    :  %v
`, anonymizeConfigFile(file, c.ShowAbspath), c.DockerBin, anonymizeWd(c.Dir, c.ShowAbspath), c.ShowAbspath)

			return nil
		},
	}
}

var (
	DESCRIPTION_CONFIG_WRITE = `Create and update gdocker configuration.
	If options are specified (--docker-bin, --dir etc.), the configuration will be updated.
	If no configuration file exists, a new one will be created with the provided options.

	Examples)
	#> gdocker config write`
)

func cmdConfigWrite() *cli.Command {
	return &cli.Command{
		Name:               "write",
		Usage:              "write gdocker configuration",
		UsageText:          ``,
		CustomHelpTemplate: TMPL_SUBCOMMAND_HELP,
		ArgsUsage:          "[options]",
		Description:        DESCRIPTION_CONFIG_WRITE,
		Before:             setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_DOCKER_BIN_DEFAULT,
			FLAG_DIRECTORY,
			FLAG_SHOW_ABSPATH,
			FLAG_CONFIG_GLOBAL,
			FLAG_VERBOSE,
			FLAG_DRYRUN,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("config write", getLogLevel(cmd.Int("verbose")))
			slog.SetDefault(logger)

			loadAndSaveConfig(cmd)

			return nil
		},
	}
}

var (
	DESCRIPTION_CONFIG_REMOVE = `Remove gdocker configuration.

	Examples)
	#> gdocker config remove`
)

func cmdConfigRemove() *cli.Command {
	return &cli.Command{
		Name:               "remove",
		Usage:              "remove gdocker configuration file",
		UsageText:          ``,
		CustomHelpTemplate: TMPL_SUBCOMMAND_HELP,
		ArgsUsage:          "[options]",
		Description:        DESCRIPTION_CONFIG_REMOVE,
		Before:             setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_CONFIG_DEFAULT,
			FLAG_VERBOSE,
			FLAG_DRYRUN,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("config rm", getLogLevel(cmd.Int("verbose")))
			slog.SetDefault(logger)

			c, file := loadConfig(cmd)
			r := bufio.NewReader(os.Stdin)
			afile := anonymizeConfigFile(file, c.ShowAbspath)
			for {
				if cmd.Bool("dry-run") {
					break
				}
				fmt.Printf("Are you sure to remove '%s'? (y/n): ", afile)
				s, _ := r.ReadString('\n')
				s = strings.TrimSpace(s)
				if s == "y" {
					if isFile(file) {
						err := os.Remove(file)
						if err != nil {
							slog.Error(err.Error())
							os.Exit(1)
						}
						fmt.Println("file removed")
					} else {
						fmt.Println("file not found")
					}
					break
				} else if s == "n" {
					break
				}
			}
			return nil
		},
	}
}
