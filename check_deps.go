package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"
)

var (
	ARGS_USAGE_SHOWDEPS  = "[options] [image names...]"
	DESCRIPTION_SHOWDEPS = `Checks and shows dependencies between images.
	This command defines a subcommand showdeps that checks and displays the dependencies between Docker images as a Mermaid flowchart. Here's a short description of the command's key components:

	Examples)
	#> gdocker showdeps --dir docker_images/arm
	#> gdocker showdeps --dir docker_images/arm --gfm`
)

var (
	TMPL_MERMAID = `{{< if .GFM >}}` + "```mermaid" + `{{< end >}}
flowchart TD
	classDef root fill:#8BA7D5,color:#000000
	classDef latest fill:#E38692,color:#000000
	classDef latestimg fill:#F6D580,color:#000000
	classDef old fill:#81D674,color:#000000
{{< range .Deps >}}
	{{< . >}}{{< end >}}
{{< if .GFM >}}` + "```" + `{{< end >}}
`
)

func cmdShowDeps() *cli.Command {
	return &cli.Command{
		Name:               "showdeps",
		Usage:              "show docker image dependencies as mermaid flowchart",
		CustomHelpTemplate: TMPL_SUBCOMMAND_HELP,
		ArgsUsage:          ARGS_USAGE_SHOWDEPS,
		Description:        DESCRIPTION_SHOWDEPS,
		Before:             setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_DIRECTORY,
			FLAG_LIST,
			FLAG_ALL,
			FLAG_ALL_LATEST,
			FLAG_GFM,
			FLAG_VERBOSE,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("showdeps", getLogLevel(cmd.Int("verbose")))
			slog.SetDefault(logger)
			dir := cmd.String("dir")

			ibds := searchImageBuildDir(dir, "archive")
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

			type tmplData struct {
				GFM  bool
				Deps []Dependency
			}

			writeTemplate(TMPL_MERMAID, tmplData{
				cmd.Bool("gfm"),
				deps_sub,
			}, "stdout", false)
			return nil
		},
	}
}

func (dep Dependency) String() string {
	if dep.From.Tag == "latest" {
		dep.To.IsLatest = true
	}
	return fmt.Sprintf("%s --> %s", printNode(dep.From), printNode(dep.To))
}

func printNode(di DockerImage) string {
	if di.IsRoot {
		return fmt.Sprintf(`%s[["%s [root]"]]:::root`, di.String(), di.String())
	}
	if di.Tag == "latest" {
		return fmt.Sprintf(`%s("%s"):::latest`, di.String(), di.String())
	}
	if di.IsLatest {
		return fmt.Sprintf(`%s("%s"):::latestimg`, di.String(), di.String())
	}
	return fmt.Sprintf(`%s("%s"):::old`, di.String(), di.String())
}
