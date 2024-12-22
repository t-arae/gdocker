package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
)

func getExistImages(dr string) []DockerImage {
	out, err := exec.Command("docker", "images", "--format", "'{{.Repository}}:{{.Tag}}'").Output()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	var images []DockerImage
	for _, v := range strings.Split(string(out), "\n") {
		i := NewDockerImage(strings.ReplaceAll(v, "'", ""))
		i.DirRoot = dr
		i.CheckDirectory()
		images = append(images, i)
	}

	return images
}

func writeCSV(cn []string, cols [][]string) {
	records := [][]string{cn}
	records = append(records, cols...)

	w := csv.NewWriter(os.Stdout)
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
			dir := cmd.String("d")

			var records [][]string
			for _, i := range getExistImages(dir) {
				records = append(records, []string{
					i.String(),
					filepath.Join(i.DirRoot, i.Dir),
					fmt.Sprintf("%t", i.ExistDir),
				})
			}
			writeCSV(
				[]string{"ImageName", "BuildDir", "Exist"},
				records,
			)

			return nil
		},
	}
}
