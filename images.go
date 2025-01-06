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
			&cli.StringFlag{
				Name:    "dir",
				Aliases: []string{"d"},
				Usage:   "directory",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("images")
			slog.SetDefault(logger)

			dir := cmd.String("dir")

			var records [][]string
			for _, i := range getExistImages(dir, "docker") {
				records = append(records, []string{
					i.String(),
					fmt.Sprintf("%t", i.ExistDir),
					filepath.Join(i.DirRoot, i.Dir),
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
func getExistImages(dr string, docker_path string) []DockerImage {
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
		img := NewDockerImage(v)
		img.DirRoot = dr
		img.CheckDirectory()
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
