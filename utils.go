package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/urfave/cli/v3"
)

// Find lines which starts with prefix from a text file
func findLines(path string, prefix string) []string {
	var results []string

	f, err := os.Open(path)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	defer f.Close()
	s := bufio.NewScanner(f)

	for s.Scan() {
		if strings.HasPrefix(s.Text(), prefix) {
			results = append(results, s.Text())
		}
	}

	if err = s.Err(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	return results
}

func setSubCommandHelpTemplate(tmpl string) func(context.Context, *cli.Command) (context.Context, error) {
	return func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
		cli.SubcommandHelpTemplate = tmpl
		return ctx, nil
	}
}

func checkImageNamesInput(cmd *cli.Command, ibds ImageBuildDirs) []string {
	var inputs []string

	if slices.Index(cmd.FlagNames(), "all") != -1 {
		if cmd.Bool("all") {
			inputs = append(inputs, ibds.ImageNames()...)
		}
	} else if slices.Index(cmd.FlagNames(), "all-latest") != -1 {
		if cmd.Bool("all-latest") {
			for _, iname := range ibds.ImageNames() {
				img, _ := NewDockerImage(iname)
				if img.Tag == "latest" {
					inputs = append(inputs, img.String())
				}
			}
		}
	} else {
		var preinputs []string
		// read image names from command line arguments.
		if cmd.NArg() > 0 {
			slog.Info("read image names from command line arguments")
			preinputs = append(preinputs, cmd.Args().Slice()...)
		}

		// Load image names from text files.
		if slices.Index(cmd.FlagNames(), "list") != -1 {
			if cmd.IsSet("list") {
				slog.Info(fmt.Sprintf("read image names from '%s'", cmd.String("list")))
				f, err := os.Open(cmd.String("list"))
				if err != nil {
					slog.Error(err.Error())
					os.Exit(1)
				}
				if len(preinputs) > 0 {
					slog.Info("append image names")
				}
				s := bufio.NewScanner(f)
				for s.Scan() {
					preinputs = append(preinputs, s.Text())
				}
				if err = s.Err(); err != nil {
					slog.Error(err.Error())
					os.Exit(1)
				}
			}
		}

		for _, preinput := range preinputs {
			img, _ := NewDockerImage(preinput)
			if img.Tag == "*" {
				if idx, ok := ibds.mapName[img.Name]; ok {
					inputs = append(inputs, ibds.ibds[idx].ImageNames()...)
				} else {
					slog.Warn(fmt.Sprintf("'%s' is not found. skipped", img))
				}
			} else {
				inputs = append(inputs, img.String())
			}
		}
	}

	if len(inputs) == 0 {
		slog.Error("please specify image name.")
		os.Exit(1)
	}

	slog.Info(fmt.Sprintf("%d images are read", len(inputs)))
	return inputs
}

func execCommand(cmd string, args []string) {
	subcmd := exec.Command(cmd, args...)
	subcmd.Stdout = os.Stdout
	subcmd.Stderr = os.Stderr
	err := subcmd.Run()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
