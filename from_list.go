package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"
)

func cmdBuildFromList() *cli.Command {
	return &cli.Command{
		Name:  "fromlist",
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
				Name:     "list",
				Aliases:  []string{"l"},
				Value:    "",
				Usage:    "list",
				Required: true,
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
			common_tag := cmd.String("t")
			list := cmd.String("list")

			deps := findDependencyFromDockerfiles(findDockerfile(dir))

			// Load image names from text files.
			var inputs []string
			if list != "" {
				f, err := os.Open(list)
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

			var image_list []DockerImage
			for _, input := range inputs {
				image := NewDockerImage(input)
				image.DirRoot = dir
				image.CheckDirectory()
				if image.ExistDir {
					image_list = append(image_list, image)
				} else {
					slog.Warn(fmt.Sprintf("'%s' is not present. skipped.", image.String()))
				}
			}
			solved := checkDependency(image_list, deps)

			var deps_sub []Dependency
			for _, i := range solved {
				d := DockerImage(i)
				for _, j := range deps {
					if d.String() == j.From.String() {
						deps_sub = append(deps_sub, j)
					}
				}
			}
			printFlowchart(deps_sub)

			for _, image := range solved {
				fmt.Print(image.BuildMakeInstruction())
				fmt.Print(image.BuildDockerTaggingInstruction(common_tag))
			}

			return nil
		},
	}
}
