package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
)

var (
	ARGS_USAGE_BUILD  = "[options] [image names...]"
	DESCRIPTION_BUILD = `Helps to run command to build Docker images.
	This command build Docker images from the list based on the specified image
	names. The command can be run in a dry-run mode to preview actions before run.

	Examples)
	#> gdocker build ubuntu_a
	#> gdocker build --list image_list.txt
	#> gdocker build -b "--platform linux/amd64" samtools_x`
)

func cmdBuild() *cli.Command {
	return &cli.Command{
		Name:               "build",
		Usage:              "build docker image from list",
		CustomHelpTemplate: TMPL_SUBCOMMAND_HELP,
		ArgsUsage:          ARGS_USAGE_BUILD,
		Description:        DESCRIPTION_BUILD,
		Before:             setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_DOCKER_BIN,
			FLAG_DIRECTORY,
			FLAG_LIST,
			FLAG_MAKEFLAG,
			FLAG_BUILDFLAG,
			FLAG_ALL,
			FLAG_ALL_LATEST,
			FLAG_SHOW_ABSPATH,
			FLAG_CONFIG_DEFAULT,
			FLAG_VERBOSE,
			FLAG_DRYRUN,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("build", getLogLevel(cmd.Int64("verbose")))
			slog.SetDefault(logger)

			config, _ := loadConfig(cmd)

			ibds := searchImageBuildDir(config.Dir, "archive")
			ibds.makeMap()
			deps := ibds.Dependencies()

			inputs := checkImageNamesInput(cmd, ibds) // load input image names from -l and args

			var images []DockerImage
			for _, input := range inputs {
				img, err := NewDockerImage(input)
				if err != nil {
					slog.Error(err.Error())
					os.Exit(1)
				}
				if _, ok := ibds.mapNameTag[img.String()]; !ok {
					slog.Warn(fmt.Sprintf("%v is not found. skipped.", img))
					continue
				}
				images = append(images, img)
			}
			solved, _ := checkDependency(images, deps)

			eimages := getExistImages(config.DockerBin)

			for _, image := range solved {
				if image.IsRoot {
					continue
				}
				if eimages.checkExist(image) {
					slog.Warn(fmt.Sprintf("%v is built. skipped.", image))
					continue
				}

				ibd := ibds.ibds[ibds.mapNameTag[image.String()]]

				version_ok, _ := ibd.MakeVersion()
				var args []string
				var args2 []string
				if !version_ok {
					args = beforeV0_0_6(image, ibd, config, cmd)
				} else {
					args, args2 = ibd.BuildMakeInstruction(image.Tag, config.ShowAbspath)
					if cmd.IsSet("build-flag") {
						args2 = append([]string{"build", cmd.String("build-flag")}, args2...)
					} else {
						args2 = append([]string{"build"}, args2...)
					}
				}

				fmt.Println("make", strings.Join(args, " "))
				if !cmd.Bool("dry-run") {
					execCommand(getWd(), "make", args)
				}

				if version_ok && image.Tag != "latest" {
					fmt.Println(config.DockerBin, strings.Join(args2, " "))
					if !cmd.Bool("dry-run") {
						execCommand(getWd(), config.DockerBin, args2)
					}
				}
			}
			return nil
		},
	}
}

func beforeV0_0_6(image DockerImage, ibd ImageBuildDir, config Config, cmd *cli.Command) (args []string) {
	slog.Warn(fmt.Sprintf("'%s' has no version. update recommended.", anonymizeWd(filepath.Join(ibd.Directory(), "Makefile"), config.ShowAbspath)))
	// Before gdocker v0.0.6, docker image building peformed by make commmand only
	args = ibd.BuildMakeInstructionOld(image.Tag, config.ShowAbspath)
	if config.DockerBin != "docker" {
		args = append(args, fmt.Sprintf("DOCKER_BIN=%s", config.DockerBin))
	}
	// add flags for make command
	if cmd.IsSet("flag") {
		args = append(args, cmd.StringSlice("flag")...)
	}
	// add flags for docker build command
	// to distiguish <v0.0.6, and >=v0.0.6, labels will be added automatically
	args = append(args, fmt.Sprintf("DOCKER_BUILD_FLAG=%s", "--label com.gdocker.version= --label com.gdocker.build-dir="))
	if cmd.IsSet("build-flag") {
		args = append(args, fmt.Sprintf("DOCKER_BUILD_FLAG=%s %s", "--label com.gdocker.version= --label com.gdocker.build-dir=", cmd.String("build-flag")))
	}
	return args
}
