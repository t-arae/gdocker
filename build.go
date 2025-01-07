package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"
)

func cmdBuild() *cli.Command {
	return &cli.Command{
		Name:  "build",
		Usage: "build docker image from list",
		Flags: []cli.Flag{
			FLAG_DIRECTORY,
			FLAG_LIST,
			FLAG_BUILDFLAG,
			FLAG_TAG,
			FLAG_VERBOSE,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("build", getLogLevel(cmd.Uint("verbose")))
			slog.SetDefault(logger)

			dir := cmd.String("dir")
			common_tag := cmd.String("tag")
			inputs := checkImageNamesInput(cmd) // load input image names from -l and args

			ibds := searchImageBuildDir(dir)
			ibds.makeMap()
			deps := ibds.findDependencyFromDockerfiles()

			var images []DockerImage
			for _, input := range inputs {
				image := NewDockerImage(input)
				if _, ok := ibds.m[image.String()]; !ok {
					slog.Warn(fmt.Sprintf("%v is not found. skipped.", image))
					continue
				}
				images = append(images, image)
			}
			solved, roots := checkDependency(images, deps)

			var deps_sub []Dependency
			for _, img := range solved {
				for _, dep := range deps {
					if img.String() == dep.From.String() {
						if _, ok := roots[dep.To.String()]; ok {
							dep.To.IsRoot = true
						}
						deps_sub = append(deps_sub, dep)
					}
				}
			}
			printFlowchart(deps_sub)

			for _, image := range solved {
				if image.IsRoot {
					continue
				}
				ibd := ibds.ibds[ibds.m[image.String()]]
				fmt.Print(ibd.BuildMakeInstruction(image.Tag))
				if IsIn(image.String(), Strings(images)) {
					fmt.Print(ibd.BuildDockerTaggingInstruction(image.Tag, common_tag))
				}
			}

			return nil
		},
	}
}

func checkImageNamesInput(cmd *cli.Command) []string {
	// read image names from command line arguments.
	var inputs []string
	if cmd.NArg() > 0 {
		slog.Info("read image names from command line arguments")
		inputs = append(inputs, cmd.Args().Slice()...)
	}

	// Load image names from text files.
	if cmd.IsSet("list") {
		slog.Info(fmt.Sprintf("read image names from '%s'", cmd.String("list")))
		f, err := os.Open(cmd.String("list"))
		if err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
		if len(inputs) > 0 {
			slog.Info("append image names")
		}
		s := bufio.NewScanner(f)
		for s.Scan() {
			inputs = append(inputs, s.Text())
		}
		if err = s.Err(); err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
	}

	if len(inputs) == 0 {
		slog.Error("please specify image name.")
		os.Exit(1)
	}

	slog.Info(fmt.Sprintf("%d images are read", len(inputs)))
	return inputs
}
