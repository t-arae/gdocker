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
	#> gdocker images
	#> gdocker images -e -b | csvtk pretty -t`
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
			FLAG_DOCKER_BIN,
			FLAG_DIRECTORY,
			FLAG_BUILT_ONLY,
			FLAG_EXIST_ONLY,
			FLAG_CONFIG_DEFAULT,
			FLAG_SHOW_ABSPATH,
			FLAG_VERBOSE,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("images", getLogLevel(cmd.Int("verbose")))
			slog.SetDefault(logger)

			config, _ := loadConfig(cmd)
			docker_bin := config.DockerBin
			dir := config.Dir

			ibds := searchImageBuildDir(dir, "archive")
			ibds.makeMap()

			eimgs := getExistImages(docker_bin)
			imgs := eimgs.Images()
			for _, iname := range ibds.ImageNames() {
				if !eimgs.checkExistByNames(iname) {
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
					fmt.Sprintf("%t", eimgs.checkExist(img)),
					fmt.Sprintf("%t", exists),
					anonymizeWd(filepath.Join(dir), config.ShowAbspath),
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

type ExistImages map[string]struct{}

// Get built docker images information
func getExistImages(docker_path string) ExistImages {
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

	exists := make(map[string]struct{})
	for _, eimg := range images {
		exists[eimg.String()] = struct{}{}
	}
	return exists
}

func (e ExistImages) checkExist(image DockerImage) bool {
	_, exist := e[image.String()]
	return exist
}

func (e ExistImages) checkExistByNames(iname string) bool {
	_, exist := e[iname]
	return exist
}

func (e ExistImages) Images() []DockerImage {
	var inames []string
	for k := range e {
		inames = append(inames, k)
	}

	var imgs []DockerImage
	for _, v := range inames {
		img, _ := NewDockerImage(v)
		imgs = append(imgs, img)
	}
	return imgs
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
