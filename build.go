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
			&cli.StringFlag{
				Name:     "dir",
				Aliases:  []string{"d"},
				Value:    "",
				Usage:    "directory",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "list",
				Aliases: []string{"l"},
				Usage:   "list",
			},
			&cli.StringFlag{
				Name:     "flag",
				Aliases:  []string{"f"},
				Value:    "",
				Usage:    "docker build flag",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "tag",
				Aliases:  []string{"t"},
				Value:    "latest",
				Usage:    "common tag name",
				Required: false,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			dir := cmd.String("d")
			deps := findDependencyFromDockerfiles(findDockerfile(dir))

			common_tag := cmd.String("t")

			// Load image names from command line arguments.
			var inputs []string
			if cmd.NArg() > 0 {
				inputs = append(inputs, cmd.Args().Slice()...)
			}

			// Load image names from text files.
			if cmd.IsSet("list") {
				f, err := os.Open(cmd.String("list"))
				if err != nil {
					slog.Error(err.Error())
					os.Exit(1)
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

			var images []DockerImage
			for _, input := range inputs {
				image := NewDockerImage(input)
				image.DirRoot = dir
				image.CheckDirectory()
				if image.ExistDir {
					images = append(images, image)
				} else {
					slog.Warn(fmt.Sprintf("'%s' is not present. skipped.", image.String()))
				}
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
				fmt.Print(image.BuildMakeInstruction())
				if IsIn(image.String(), Strings(images)) {
					fmt.Print(image.BuildDockerTaggingInstruction(common_tag))
				}
			}

			return nil
		},
	}
}

func IsIn[T comparable](x T, s []T) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}
