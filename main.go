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

const APP_NAME = "gdocker"

func main() {
	cmd := &cli.Command{}
	cmd.Name = APP_NAME
	cmd.Usage = "docker utility"
	cmd.Version = fmt.Sprintf("0.0.1 (%s)\n", getDockerVersion("docker"))

	logger := getLogger("main")
	slog.SetDefault(logger)

	cmd.Commands = []*cli.Command{
		cmdShowDeps(),
		cmdBuild(),
		cmdRun(),
		cmdRunWorkingDirectory(),
		cmdImages(),
		cmdTemplate(),
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

// Get docker version string
func getDockerVersion(docker_path string) string {
	out, err := exec.Command(docker_path, "--version").CombinedOutput()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	return strings.TrimSpace(string(out))
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

func getLogger(name string) *slog.Logger {
	sh := NewSimpleHandler(os.Stdout, slog.LevelInfo, name)
	return slog.New(sh)
}
