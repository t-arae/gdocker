package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/urfave/cli/v3"
)

var (
	APP_NAME    = "gdocker"
	APP_USAGE   = "personal docker wrapper tool written in Go"
	APP_VERSION = "0.0.1"
)

func cmdMain() *cli.Command {
	cmd := &cli.Command{}
	cmd.Name = APP_NAME
	cmd.Usage = APP_USAGE
	cmd.Flags = []cli.Flag{
		FLAG_VERBOSE,
		FLAG_DOCKER_BIN,
		FLAG_CONFIG_DEFAULT,
	}
	cmd.Version = fmt.Sprintf("%s %s", APP_VERSION, getDockerVersion("docker"))
	cmd.Before = func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
		logger := getLogger("main", getLogLevel(cmd.Int("verbose")))
		slog.SetDefault(logger)

		config, err := readConfig(searchConfigFiles(cmd.StringSlice("config")))
		if err != nil {
			slog.Warn(err.Error())
			config.updateDockerBin("docker")
		}

		config.updateDockerBin(cmd.String("docker-bin"))
		docker_bin := config.DockerBin

		cmd.Version = fmt.Sprintf("%s %s", APP_VERSION, getDockerVersion(docker_bin))
		return ctx, nil
	}

	cmd.Commands = []*cli.Command{
		cmdShowDeps(),
		cmdBuild(),
		cmdClean(),
		cmdImages(),
		cmdRun(),
		cmdRunWorkingDirectory(),
		cmdDev(),
	}
	return cmd
}

func main() {
	cmd := cmdMain()
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

// Get docker version string
func getDockerVersion(docker_path string) string {
	out, err := exec.Command(docker_path, "--version").CombinedOutput()
	if err != nil {
		slog.Info(err.Error())
		return "(could not get docker version)"
	}
	return fmt.Sprintf("(%s)", strings.TrimSpace(string(out)))
}

// logger setup
type SimpleHandler struct {
	slog.Handler
	logger *log.Logger
	name   string
}

func NewSimpleHandler(out io.Writer, level slog.Level, name string) *SimpleHandler {
	prefix := ""
	h := &SimpleHandler{
		Handler: slog.NewTextHandler(out, &slog.HandlerOptions{
			Level: level,
		}),
		logger: log.New(out, prefix, 0),
		name:   name,
	}
	return h
}

func (h *SimpleHandler) Handle(_ context.Context, record slog.Record) error {
	ts := record.Time.Format("[2006-01-02 15:04:05]")
	level := fmt.Sprintf("[%5s]", record.Level.String())
	h.logger.Println(ts, h.name, level, record.Message)
	return nil
}

func getLogger(name string, level slog.Level) *slog.Logger {
	sh := NewSimpleHandler(os.Stdout, level, name)
	return slog.New(sh)
}

func getLogLevel(i int64) slog.Level {
	var lev slog.Level
	switch i {
	case -1:
		lev = slog.LevelDebug
	case 0:
		lev = slog.LevelInfo
	case 1:
		lev = slog.LevelWarn
	case 2:
		lev = slog.LevelError
	default:
		lev = slog.LevelInfo
	}
	return lev
}
