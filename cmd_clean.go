package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/urfave/cli/v3"
)

var (
	ARGS_USAGE_CLEAN  = "[options] [image names...]"
	DESCRIPTION_CLEAN = `Helps to run command to remove Docker images.
	This command removes Docker images from the list based on the specified image
	names. The command can be run in a dry-run mode to preview actions before run.

	Examples)
	#> gdocker clean ubuntu_a
	#> gdocker clean ubuntu_a:*
	#> gdocker clean --list image_list.txt
	#> gdocker clean --all -n`
)

func cmdClean() *cli.Command {
	return &cli.Command{
		Name:               "clean",
		Usage:              "clean docker image from list",
		CustomHelpTemplate: TMPL_SUBCOMMAND_HELP,
		ArgsUsage:          ARGS_USAGE_CLEAN,
		Description:        DESCRIPTION_CLEAN,
		Before:             setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_DOCKER_BIN,
			FLAG_DIRECTORY,
			FLAG_LIST,
			FLAG_MAKEFLAG,
			FLAG_ALL,
			FLAG_SHOW_ABSPATH,
			FLAG_CONFIG_DEFAULT,
			FLAG_VERBOSE,
			FLAG_DRYRUN,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("clean", getLogLevel(cmd.Int("verbose")))
			slog.SetDefault(logger)

			config, _ := loadConfig(cmd)
			docker_bin := config.DockerBin
			dir := config.Dir

			var flags []string
			if docker_bin != "docker" {
				flags = append(flags, fmt.Sprintf("DOCKER_BIN=%s", docker_bin))
			}
			flags = append(flags, cmd.StringSlice("flag")...)

			ibds := searchImageBuildDir(dir, "archive")
			ibds.makeMap()

			inputs := checkImageNamesInput(cmd, ibds) // load input image names from -l and args

			eimages := getExistImages(docker_bin)

			finished := make(map[string]struct{})
			for _, input := range inputs {
				image, err := NewDockerImage(input)
				if err != nil {
					slog.Error(err.Error())
					os.Exit(1)
				}

				if idx, ok := ibds.mapNameTag[image.String()]; ok {
					if _, isfinish := finished[image.String()]; isfinish {
						continue
					}

					if !eimages.checkExist(image) {
						slog.Warn(fmt.Sprintf("%v is not built. skipped.", image))
						continue
					}

					args := ibds.ibds[idx].BuildCleanInstruction(image.Tag, config.ShowAbspath)
					args = append(args, flags...)
					fmt.Println("make", strings.Join(args, " "))
					if !cmd.Bool("dry-run") {
						execCommand("make", args)
					}
					finished[image.String()] = struct{}{}
				}
			}

			return nil
		},
	}
}
