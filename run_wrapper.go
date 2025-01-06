package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"syscall"

	"github.com/urfave/cli/v3"

	"github.com/creack/pty"
	"golang.org/x/term"
)

func cmdRun() *cli.Command {
	return &cli.Command{
		SkipFlagParsing: true,
		Name:            "run",
		Usage:           "docker run with uid and gid",
		ArgsUsage:       "[arguments...]",
		CustomHelpTemplate: `NAME:
	{{template "helpNameTemplate" .}}
USAGE:
	{{.FullName}} {{.ArgsUsage}}
`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("run")
			slog.SetDefault(logger)

			var ca cmdArgs
			ca.wd.Skip = true

			cmdargs := ca.buildCmdArgs(cmd.Args().Slice())

			docker_path := "docker"

			if !IsIn("-it", cmdargs) {
				subcmd := exec.Command(docker_path, cmdargs...)
				subcmd.Stdout = os.Stdout
				subcmd.Stderr = os.Stderr
				err := subcmd.Run()
				if err != nil {
					slog.Error(err.Error())
					os.Exit(1)
				}
			} else {
				subcmd := exec.Command(docker_path, cmdargs...)
				err := startPty(subcmd)
				if err != nil {
					slog.Error(err.Error())
					os.Exit(1)
				}
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
		ArgsUsage:       "[arguments...]",
		CustomHelpTemplate: `NAME:
	{{template "helpNameTemplate" .}}
USAGE:
	{{.FullName}} {{.ArgsUsage}}
`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("wdrun")
			slog.SetDefault(logger)

			var ca cmdArgs

			cmdargs := ca.buildCmdArgs(cmd.Args().Slice())

			docker_path := "docker"

			if !IsIn("-it", cmdargs) {
				subcmd := exec.Command(docker_path, cmdargs...)
				subcmd.Stdout = os.Stdout
				subcmd.Stderr = os.Stderr
				err := subcmd.Run()
				if err != nil {
					slog.Error(err.Error())
					os.Exit(1)
				}
			} else {
				subcmd := exec.Command(docker_path, cmdargs...)
				err := startPty(subcmd)
				if err != nil {
					slog.Error(err.Error())
					os.Exit(1)
				}
			}
			return nil
		},
	}
}

// Working directoryのパスを返す
func getWd() string {
	wd, err := os.Getwd()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	return wd
}

// user IDとgroup IDを返す
func getIds() (string, string) {
	u, err := user.Current()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	return u.Uid, u.Gid
}

// docker runのコマンドライン引数の値を表す。Skip = trueの場合はスキップする。
type argMember struct {
	Value string
	IsSet bool
	Skip  bool
}

func (m *argMember) Set(v string) {
	m.Value = v
	m.IsSet = true
}

// 自動設定したいdocker runのコマンドライン引数群を表す
type cmdArgs struct {
	wd  argMember
	uid argMember
	gid argMember
}

// cmdArgsの設定すべき引数を自動設定し、コマンドライン引数の文字列のスライスを返す
func (ca *cmdArgs) buildCmdArgs(cmds []string) []string {
	if !ca.wd.IsSet {
		ca.wd.Set(fmt.Sprintf("%s:/data", getWd()))
	}

	uid, gid := getIds()
	if !ca.uid.IsSet {
		ca.uid.Set(fmt.Sprintf("LOCAL_UID=%s", uid))
	}
	if !ca.gid.IsSet {
		ca.gid.Set(fmt.Sprintf("LOCAL_GID=%s", gid))
	}

	args := []string{"run", "--rm"}
	if !ca.wd.Skip {
		args = append(args, "-v", ca.wd.Value)
	}
	if !ca.uid.Skip {
		args = append(args, "-e", ca.uid.Value)
	}
	if !ca.gid.Skip {
		args = append(args, "-e", ca.gid.Value)
	}

	for _, cmd := range cmds {
		args = append(args, cmd)
	}
	return args
}

func startPty(cmd *exec.Cmd) error {
	// Start the command with a pty.
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.

	// Handle pty size.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				slog.Error(fmt.Sprintf("error resizing pty: %s", err))
			}
		}
	}()
	ch <- syscall.SIGWINCH                        // Initial resize.
	defer func() { signal.Stop(ch); close(ch) }() // Cleanup signals when done.

	// Set stdin in raw mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }() // Best effort.

	// Copy stdin to the pty and the pty to stdout.
	// NOTE: The goroutine will keep reading until the next keystroke before returning.
	go func() { _, _ = io.Copy(ptmx, os.Stdin) }()
	_, _ = io.Copy(os.Stdout, ptmx)

	return nil
}
