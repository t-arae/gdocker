package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/urfave/cli/v3"
)

func cmdShowDeps() *cli.Command {
	return &cli.Command{
		Name:  "showdeps",
		Usage: "show docker image dependencies as mermaid flowchart",
		Flags: []cli.Flag{
			FLAG_DIRECTORY,
			FLAG_GFM,
			FLAG_VERBOSE,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("showdeps", getLogLevel(cmd.Uint("verbose")))
			slog.SetDefault(logger)

			dir := cmd.String("dir")
			if cmd.Bool("gfm") {
				fmt.Println("```mermaid")
			}
			deps := searchImageBuildDir(dir).findDependencyFromDockerfiles()
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
