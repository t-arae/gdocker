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
	ARGS_USAGE_TAG  = "[options] [image names...]"
	DESCRIPTION_TAG = `Tags/Untags Docker images with a project-specific tag.
	This command applies the project specific tag (specified by '-t, --proj-tag')
	to the given images. Only images that already exist are processed. Also, images
	already tagged as 'latest' are skipped.

	Examples)
	# Tag a single image with a project tag
	#> gdocker tag -t awesome ubuntu_a:22.04
	# Preview the docker commands without executing them
	#> gdocker tag -t awesome -n ubuntu_a:22.04
	# Untag (remove) docker image
	#> gdocker tag -t awesome -u ubuntu_a
`
)

func cmdTag() *cli.Command {
	return &cli.Command{
		Name:               "tag",
		Usage:              "tag/untag images with specified project tag",
		CustomHelpTemplate: TMPL_SUBCOMMAND_HELP,
		ArgsUsage:          ARGS_USAGE_TAG,
		Description:        DESCRIPTION_TAG,
		Before:             setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_PROJ_TAG,
			FLAG_UNTAG,
			FLAG_DOCKER_BIN,
			FLAG_DIRECTORY,
			FLAG_LIST,
			FLAG_MAKEFLAG,
			FLAG_ALL,
			FLAG_CONFIG_DEFAULT,
			FLAG_VERBOSE,
			FLAG_DRYRUN,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("tag", getLogLevel(cmd.Int64("verbose")))
			slog.SetDefault(logger)

			config, _ := loadConfig(cmd)
			docker_bin := config.DockerBin
			dir := config.Dir
			projtag := config.ProjectTag

			if projtag == "latest" {
				slog.Info("the project tag is 'latest'. skipped.")
				return nil
			}

			ibds := searchImageBuildDir(dir, "archive")
			ibds.makeMap()

			// load input image names from -l and args
			inputs := checkImageNamesInput(cmd, ibds)

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

					if image.Tag == "latest" {
						continue
					}

					if cmd.Bool("untag") {
						args := ibds.ibds[idx].BuildRemoveImage(projtag)
						fmt.Println(docker_bin, strings.Join(args, " "))
						if !cmd.Bool("dry-run") {
							execCommand(getWd(), docker_bin, args)
						}
					} else {
						args, use := ibds.ibds[idx].BuildTaggingInstruction(image.Tag, projtag)
						if !use {
							continue
						}

						fmt.Println(docker_bin, strings.Join(args, " "))
						if !cmd.Bool("dry-run") {
							execCommand(getWd(), docker_bin, args)
						}
					}
					finished[image.String()] = struct{}{}
				}
			}
			return nil
		},
	}
}
