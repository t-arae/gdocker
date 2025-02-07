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
	DESCRIPTION_CLEAN = `Helps to run commnad to remove Docker images.
	This command removes Docker images from the list based on the specified image
	names. The command can be run in a dry-run mode to preview actions before run.
	Note: the action removes all images which matches image names regardless of its tag.

	Examples)
	#> gdocker clean --dir docker_images/arm ubuntu_a
	#> gdocker clean --dir docker_images/arm --list image_list.txt
	#> gdocker clean --dir docker_images/arm --all -n`
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
			FLAG_DIRECTORY,
			FLAG_LIST,
			FLAG_ALL,
			FLAG_DRYRUN,
			FLAG_DOCKER_BIN,
			FLAG_VERBOSE,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("clean", getLogLevel(cmd.Int("verbose")))
			slog.SetDefault(logger)

			dir := cmd.String("dir")

			ibds := searchImageBuildDir(dir, "archive")
			ibds.makeMap()

			inputs := checkImageNamesInput(cmd, ibds) // load input image names from -l and args

			finished := make(map[int]struct{})
			for _, input := range inputs {
				image, err := NewDockerImage(input)
				if err != nil {
					slog.Error(err.Error())
					os.Exit(1)
				}
				if idx, ok := ibds.mapNameTag[image.String()]; ok {
					if _, isfinish := finished[idx]; !isfinish {
						args := ibds.ibds[idx].BuildCleanInstruction()
						if cmd.String("docker-bin") != "docker" {
							args = append(args, fmt.Sprintf("DOCKER_BIN=%s", cmd.String("docker-bin")))
						}
						fmt.Println("make", strings.Join(args, " "))
						if !cmd.Bool("dry-run") {
							execCommand("make", args)
						}
						finished[idx] = struct{}{}
					}
				}
			}

			return nil
		},
	}
}
