package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
)

var (
	// flag for images command
	FLAG_BUILT_ONLY = &cli.BoolFlag{
		Name:    "built-only",
		Aliases: []string{"b"},
		Value:   false,
		Usage:   "show only images already built",
	}
	FLAG_EXIST_ONLY = &cli.BoolFlag{
		Name:    "exist-only",
		Aliases: []string{"e"},
		Value:   false,
		Usage:   "show only images with building directory",
	}
)

var (
	ARGS_USAGE_IMAGES  = "[options]"
	DESCRIPTION_IMAGES = `Shows docker images have been built with some additional infomation.
	This command lists Docker images that have already been built, showing their
	build status and associated directories. It supports filtering to display only
	built images or those with a build directory. The output is provided in TSV format.

	Examples)
	#> gdocker images --dir docker_images/arm

	(bellow examples needs "csvtk" to run.)
	#> gdocker images --dir docker_images/arm | csvtk pretty -t
	#> gdocker images --dir docker_images/arm -e -b | csvtk pretty -t`
)

func cmdImages() *cli.Command {
	return &cli.Command{
		Name:               "images",
		Usage:              "show built images with some info",
		CustomHelpTemplate: TMPL_SUBCOMMAND_HELP,
		ArgsUsage:          ARGS_USAGE_IMAGES,
		Description:        DESCRIPTION_IMAGES,
		Before:             setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_DIRECTORY,
			FLAG_BUILT_ONLY,
			FLAG_EXIST_ONLY,
			FLAG_VERBOSE,
			FLAG_DOCKER_BIN,
			FLAG_CONFIG_DEFAULT,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("images", getLogLevel(cmd.Int("verbose")))
			slog.SetDefault(logger)

			config := loadConfig(cmd)
			docker_bin := config.DockerBin
			dir := config.Dir

			ibds := searchImageBuildDir(dir, "archive")
			ibds.makeMap()

			imgs := getExistImages(docker_bin)
			imgs_map := make(map[string]struct{})
			for _, img := range imgs {
				imgs_map[img.String()] = struct{}{}
			}
			for _, iname := range ibds.ImageNames() {
				if _, ok := imgs_map[iname]; !ok {
					img, err := NewDockerImage(iname)
					if err != nil {
						slog.Error(err.Error())
						os.Exit(1)
					}
					imgs = append(imgs, img)
				}
			}

			var records [][]string
			for _, img := range imgs {
				idx, exists := ibds.mapNameTag[img.String()]
				_, built := imgs_map[img.String()]
				var dir string
				if exists {
					if img.Tag == "latest" {
						dir = filepath.Join(ibds.ibds[idx].dirParent, img.Name)
					} else {
						dir = filepath.Join(ibds.ibds[idx].dirParent, img.Name, img.Tag)
					}
				} else {
					dir = ""
				}
				records = append(records, []string{
					img.String(),
					fmt.Sprintf("%t", built),
					fmt.Sprintf("%t", exists),
					filepath.Join(dir),
				})
			}

			if cmd.Bool("built-only") {
				var filtered [][]string
				for _, record := range records {
					if record[1] == "true" {
						filtered = append(filtered, record)
					}
				}
				records = filtered
			}

			if cmd.Bool("exist-only") {
				var filtered [][]string
				for _, record := range records {
					if record[2] == "true" {
						filtered = append(filtered, record)
					}
				}
				records = filtered
			}

			writeCSV(
				[]string{"ImageName", "Built", "Exist", "BuildDir"},
				records,
				os.Stdout,
			)

			return nil
		},
	}
}

// Get built docker images information
func getExistImages(docker_path string) []DockerImage {
	out, err := exec.Command(docker_path, "images", "--format", "{{.Repository}}:{{.Tag}}").Output()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	var images []DockerImage
	for _, v := range strings.Split(string(out), "\n") {
		if v == "" || v == "<none>:<none>" {
			continue
		}
		// this parse error is ignored intentionally.
		img, _ := NewDockerImage(v)
		images = append(images, img)
	}

	return images
}

func writeCSV(cn []string, cols [][]string, wo io.Writer) {
	records := [][]string{cn}
	records = append(records, cols...)

	w := csv.NewWriter(wo)
	w.Comma = '\t'
	for _, record := range records {
		if err := w.Write(record); err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
	}
	w.Flush()

	if err := w.Error(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
