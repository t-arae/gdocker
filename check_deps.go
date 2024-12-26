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
			&cli.BoolFlag{
				Name:    "gfm",
				Aliases: []string{"m"},
				Value:   false,
				Usage:   "print for GitHub Fravored Markdown",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			dir := cmd.String("dir")
			if cmd.Bool("gfm") {
				fmt.Println("```mermaid")
			}
			deps := findDependencyFromDockerfiles(searchImageBuildDir(dir))
			printFlowchart(deps)
			if cmd.Bool("gfm") {
				fmt.Println("```")
			}
			return nil
		},
	}
}

func printFlowchart(deps []Dependency) {
	fmt.Println("flowchart TD")
	for _, dep := range deps {
		fmt.Printf("\t%s --> %s\n", printNode(dep.From), printNode(dep.To))
	}
}

func printNode(di DockerImage) string {
	if di.IsRoot {
		return fmt.Sprintf(`%s[["%s [root]"]]`, di.String(), di.String())
	}
	return fmt.Sprintf(`%s("%s")`, di.String(), di.String())
}
