package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"slices"
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
			logger := getLogger("images", getLogLevel(cmd.Int64("verbose")))
			slog.SetDefault(logger)

			config, _ := loadConfig(cmd)
			docker_bin := config.DockerBin
			dir := config.Dir

			ibds := searchImageBuildDir(dir, "archive")
			ibds.makeMap()

			iis := getImageInfo(docker_bin)
			m := getMapExistImageNames(iis)

			var records [][]string
			for _, ii := range iis {
				record := ii.ToRecord()
				if _, ok := ibds.mapNameTag[record[0][0]]; !ok {
					for i := range record {
						record[i][2] = "false"
					}
				}
				records = append(records, record...)
			}

			for _, iname := range ibds.ImageNames() {
				if i, ok := m[iname]; ok {
					records[i][1] = "true"
				} else {
					records = append(records, []string{iname, "false", "true", "", ""})
				}
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

			if !config.ShowAbspath {
				for i := range records {
					if records[i][4] != "" {
						records[i][4] = anonymizeWd(records[i][4], false)
					}
				}
			}

			writeCSV(
				[]string{"ImageName", "Built", "Exist", "Version", "BuildDir"},
				records,
				os.Stdout,
			)

			return nil
		},
	}
}

type ImageInfo struct {
	Hash   string            `json:"hash"`
	Names  []string          `json:"name"`
	Labels map[string]string `json:"label"`
}

func (ii *ImageInfo) ToRecord() [][]string {
	var records [][]string
	for _, in := range ii.Names {
		record := []string{in, "true", "true", ii.gdockerVersion(), ii.buildDir()}
		records = append(records, record)
	}
	return records
}

func (ii *ImageInfo) gdockerVersion() string {
	if v, ok := ii.Labels["com.gdocker.version"]; ok {
		return v
	}
	return ""
}

func (ii *ImageInfo) buildDir() string {
	if v, ok := ii.Labels["com.gdocker.build-dir"]; ok {
		return v
	}
	return ""
}

func getMapExistImageNames(iis []ImageInfo) map[string]int {
	m := make(map[string]int)
	for _, ii := range iis {
		for i, iname := range ii.Names {
			m[iname] = i
		}
	}
	return m
}

func getImageInfo(docker_path string) []ImageInfo {
	out, err := exec.Command(docker_path, "images", "--filter", "dangling=false", "--format", "{{.ID}}").Output()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	images := strings.Split(string(out), "\n")
	slices.Sort(images)
	images = slices.DeleteFunc(images, func(e string) bool {
		return e == ""
	})
	images = slices.Compact(images)
	out, err = exec.Command(docker_path, append([]string{"image", "inspect", "--format", "{ \"hash\" : {{json .Id}},  \"name\" : {{json .RepoTags}}, \"label\" : {{json .Config.Labels}} }"}, images...)...).Output()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	lines := slices.DeleteFunc(bytes.Split(out, []byte("\n")), func(b []byte) bool {
		return len(b) == 0
	})
	recs := make([]ImageInfo, len(lines))
	for i, line := range lines {
		if len(line) == 0 {
			continue
		}
		err = json.Unmarshal(line, &recs[i])
		if err != nil {
			slog.Error(err.Error())
		}
	}
	return recs
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
