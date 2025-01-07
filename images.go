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

func cmdImages() *cli.Command {
	return &cli.Command{
		Name:  "images",
		Usage: "test",
		Flags: []cli.Flag{
			FLAG_DIRECTORY,
			FLAG_VERBOSE,
			FLAG_DOCKER_BIN,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("images", getLogLevel(cmd.Uint("verbose")))
			slog.SetDefault(logger)

			dir := cmd.String("dir")
			ibds := searchImageBuildDir(dir)
			ibds.makeMap()

			var records [][]string
			for _, i := range getExistImages(cmd.String("docker-bin")) {
				idx, exists := ibds.m[i.String()]
				var dir string
				if exists {
					dir = filepath.Join(ibds.ibds[idx].dirParent, i.Name, i.Tag)
				} else {
					dir = ""
				}
				records = append(records, []string{
					i.String(),
					fmt.Sprintf("%t", exists),
					filepath.Join(dir),
				})
			}
			writeCSV(
				[]string{"ImageName", "Exist", "BuildDir"},
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
		if v == "" {
			continue
		}
		images = append(images, NewDockerImage(v))
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
