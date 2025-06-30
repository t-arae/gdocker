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
	#> gdocker clean --dir docker_images/arm ubuntu_a
	#> gdocker clean --dir docker_images/arm ubuntu_a:*
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
			FLAG_MAKEFLAG,
			FLAG_ALL,
			FLAG_DRYRUN,
			FLAG_DOCKER_BIN,
			FLAG_VERBOSE,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("clean", getLogLevel(cmd.Int("verbose")))
			slog.SetDefault(logger)

			dir := cmd.String("dir")
			var flags []string
			if cmd.String("docker-bin") != "docker" {
				flags = append(flags, fmt.Sprintf("DOCKER_BIN=%s", cmd.String("docker-bin")))
			}
			flags = append(flags, cmd.StringSlice("flag")...)

			ibds := searchImageBuildDir(dir, "archive")
			ibds.makeMap()

			inputs := checkImageNamesInput(cmd, ibds) // load input image names from -l and args

			existsImages := make(map[string]struct{})
			for _, eimg := range getExistImages(cmd.String("docker-bin")) {
				existsImages[eimg.String()] = struct{}{}
			}

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

					if _, isbuilt := existsImages[image.String()]; !isbuilt {
						slog.Warn(fmt.Sprintf("%v is not built. skipped.", image))
						continue
					}

					args := ibds.ibds[idx].BuildCleanInstruction(image.Tag)
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
