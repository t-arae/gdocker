package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func cmdShowDeps() *cli.Command {
	return &cli.Command{
		Name:  "showdeps",
		Usage: "show docker image dependencies as mermaid flowchart",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "dir",
				Aliases:  []string{"d"},
				Value:    "",
				Usage:    "directory",
				Required: true,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			dir := cmd.String("d")
			printFlowchart(findDependencyFromDockerfiles(findDockerfile(dir)))
			return nil
		},
	}
}

func printFlowchart(deps []Dependency) {
	fmt.Println("flowchart TD")
	for _, dep := range deps {
		fmt.Printf("\t%v --> %v\n", dep.From, dep.To)
	}
}
