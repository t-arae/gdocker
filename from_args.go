package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/urfave/cli/v3"
)

func cmdBuildFromArgs() *cli.Command {
	return &cli.Command{
		Name:  "fromarg",
		Usage: "build docker image from args",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "dir",
				Aliases:  []string{"d"},
				Value:    "",
				Usage:    "directory",
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
			inputs := cmd.Args().Slice() // Load image names from command line arguments.

			deps := findDependencyFromDockerfiles(findDockerfile(dir))

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
