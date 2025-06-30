package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/urfave/cli/v3"
)

func cmdBuild() *cli.Command {
	return &cli.Command{
		Name:   "build",
		Usage:  "build docker image from list",
		Before: setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_DIRECTORY,
			FLAG_LIST,
			FLAG_BUILDFLAG,
			FLAG_TAG,
			FLAG_ALL,
			FLAG_ALL_LATEST,
			FLAG_DRYRUN,
			FLAG_VERBOSE,
			FLAG_DOCKER_BIN,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("build", getLogLevel(cmd.Int("verbose")))
			slog.SetDefault(logger)

			dir := cmd.String("dir")
			common_tag := cmd.String("tag")
			bflag := cmd.String("flag")

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
			tmpl := NewTemplates(TMPL_MERMAID, tmplData{
				cmd.Bool("gfm"),
				deps_sub,
			})
			tmpl.writeTemplates("stdout", false)

			for _, image := range solved {
				if image.IsRoot {
					continue
				}
				ibd := ibds.ibds[ibds.mapNameTag[image.String()]]

				args := ibd.BuildMakeInstruction(image.Tag)
				if cmd.String("docker-bin") != "docker" {
					args = append(args, fmt.Sprintf("DOCKER_BIN=%s", cmd.String("docker-bin")))
				}
				if bflag != "" {
					args = append(args, fmt.Sprintf("DOCKER_BUILD_FLAG=%s", bflag))
				}
				fmt.Println("make", strings.Join(args, " "))
				if !cmd.Bool("dry-run") {
					execCommand("make", args)
				}
				if slices.Index(Strings(images), image.String()) != -1 {
					args, ok := ibd.BuildTaggingInstruction(image.Tag, common_tag)
					if ok {
						fmt.Print(cmd.String("docker-bin"), strings.Join(args, " "))
					}
					if !cmd.Bool("dry-run") && ok {
						execCommand(cmd.String("docker-bin"), args)
					}
				}
			}
			return nil
		},
	}
}
