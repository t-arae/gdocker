package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/urfave/cli/v3"
)

const APP_NAME = "gdocker"

func main() {
	cmd := &cli.Command{}
	cmd.Name = APP_NAME
	cmd.Usage = "docker utility"
	cmd.Version = fmt.Sprintf("0.0.1 (%s)\n", getDockerVersion())

	logger := slog.Default()
	slog.SetDefault(logger)

	cmd.Commands = []*cli.Command{
		cmdShowDeps(),
		cmdBuildFromArgs(),
		cmdBuildFromList(),
		cmdRun(),
		cmdRunWorkingDirectory(),
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
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
