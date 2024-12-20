package main

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/urfave/cli/v3"
)

func cmdRun() *cli.Command {
	return &cli.Command{
		SkipFlagParsing: true,
		Name:            "run",
		Usage:           "docker run with uid and gid",
		Action: func(ctx context.Context, cmd *cli.Command) error {

			var ca cmdArgs
			ca.wd.Skip = true

			subcmd := exec.Command("docker", ca.buildCmdArgs(cmd.Args().Slice())...)
			subcmd.Stdout = os.Stdout
			subcmd.Stderr = os.Stderr
			err := subcmd.Run()
			if err != nil {
				panic(err)
			}

			return nil
		},
	}
}

func cmdRunWorkingDirectory() *cli.Command {
	return &cli.Command{
		SkipFlagParsing: true,
		Name:            "wdrun",
		Usage:           "docker run with uid, gid and working directory",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var ca cmdArgs

			subcmd := exec.Command("docker", ca.buildCmdArgs(cmd.Args().Slice())...)
			subcmd.Stdout = os.Stdout
			subcmd.Stderr = os.Stderr
			err := subcmd.Run()
			if err != nil {
				panic(err)
			}

			return nil
		},
	}
}

func getDockerVersion() string {
	out, err := exec.Command("docker", "--version").CombinedOutput()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	return strings.TrimSpace(string(out))
}
