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
			dir := cmd.String("dir")

			ibds := searchImageBuildDir(dir)
			ibds_map := make(map[string]int)
			for i, ibd := range ibds {
				for _, iname := range ibd.ImageNames() {
					ibds_map[iname] = i
				}
			}
			deps := findDependencyFromDockerfiles(ibds)

			common_tag := cmd.String("tag")

			inputs := checkImageNamesInput(cmd)

			var images []DockerImage
			for _, input := range inputs {
				image := NewDockerImage(input)
				if _, ok := ibds_map[image.String()]; !ok {
					slog.Warn(fmt.Sprintf("%v is not found. skipped.", image))
					continue
				}
				image.DirRoot = dir
				image.CheckDirectory()
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
				fmt.Print(image.BuildMakeInstruction())
				if IsIn(image.String(), Strings(images)) {
					fmt.Print(image.BuildDockerTaggingInstruction(common_tag))
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

// Check whether the element x is in the slice s
func IsIn[T comparable](x T, s []T) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}
