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
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/urfave/cli/v3"

	"github.com/creack/pty"
	"golang.org/x/term"
)

var (
	ARGS_USAGE_RUN  = "[options ---] arguments..."
	DESCRIPTION_RUN = `A "docker run" wrapper with set some environment variables and flags.
  - set automatic container removal ("--rm")
  - mount current working directory to /data (-v {PWD}:/data) (only "wdrun")
  - current User/Group ID (through "LOCAL_UID" and "LOCAL_GID")
  - disable showing startup message (through "ECHO_IDS")

When you use "--verbose" or "--docker-bin" option, you have to write " --- " before
arguments. If you pass the "-it" to arguments, you can use interactive mode.

Examples)
#> gdocker run ubuntu_a uname -a
#> gdocker run --verbose 0 --- ubuntu_a uname -a
#> gdocker wdrun -it ubuntu_a bash`
)

func cmdRun() *cli.Command {
	return &cli.Command{
		SkipFlagParsing: true,
		Name:            "run",
		Usage:           "docker run with uid and gid",
		Flags: []cli.Flag{
			FLAG_DOCKER_BIN,
			FLAG_CONFIG_DEFAULT,
			FLAG_VERBOSE,
		},
		ArgsUsage:   ARGS_USAGE_RUN,
		Description: DESCRIPTION_RUN,

		Action: func(ctx context.Context, cmd *cli.Command) error {
			var ca cmdArgs
			ca.wd.Skip = true

			args, isHelp, config, lev := parseRunArgs(cmd.Args().Slice())
			docker_path := config.DockerBin
			if isHelp {
				cli.HelpPrinter(os.Stdout, cli.SubcommandHelpTemplate, cmd)
				return nil
			}
			cmdargs := ca.buildCmdArgs(args)
			logger := getLogger("run", lev)
			slog.SetDefault(logger)

			if slices.Index(cmdargs, "-it") == -1 {
				slog.Info(fmt.Sprintf("command is '%s %s'", docker_path, strings.Join(cmdargs, " ")))
				execCommand(docker_path, cmdargs)
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
		Flags: []cli.Flag{
			FLAG_DOCKER_BIN,
			FLAG_CONFIG_DEFAULT,
			FLAG_VERBOSE,
		},
		ArgsUsage:   ARGS_USAGE_RUN,
		Description: DESCRIPTION_RUN,

		Action: func(ctx context.Context, cmd *cli.Command) error {
			var ca cmdArgs

			args, isHelp, config, lev := parseRunArgs(cmd.Args().Slice())
			docker_path := config.DockerBin
			if isHelp {
				cli.HelpPrinter(os.Stdout, cli.SubcommandHelpTemplate, cmd)
				return nil
			}
			cmdargs := ca.buildCmdArgs(args)
			logger := getLogger("wdrun", lev)
			slog.SetDefault(logger)

			if slices.Index(cmdargs, "-it") == -1 {
				slog.Info(fmt.Sprintf("command is '%s %s'", docker_path, strings.Join(cmdargs, " ")))
				execCommand(docker_path, cmdargs)
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

func parseRunArgs(args []string) ([]string, bool, Config, slog.Level) {
	lev := slog.LevelWarn

	if len(args) == 1 && slices.Index([]string{"--help", "-h"}, args[0]) != -1 {
		return []string{}, true, Config{}, lev
	}

	config, err := readConfig(searchConfigFiles(FLAG_CONFIG_DEFAULT.Value))
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	if i := slices.Index(args, "---"); i != -1 {
		gdargs := slices.Clone(args[0:i])
		args = slices.Delete(args, 0, i+1)

		if i_V := slices.IndexFunc(gdargs, func(e string) bool { return e == "--verbose" || e == "-V" }); i_V != -1 {
			if i_V+1 < len(gdargs) {
				i, _ := strconv.ParseInt(gdargs[i_V+1], 10, 64)
				lev = getLogLevel(i)
			}
		}

		slog.SetDefault(getLogger("run", lev))

		var config_files []string
		for i, arg := range gdargs {
			if strings.HasPrefix(arg, "--config") {
				config_files = append(config_files, gdargs[i+1])
			}
		}
		if len(config_files) != 0 {
			config, err = readConfig(searchConfigFiles(config_files))
			if err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
		}

		if i_bin := slices.Index(gdargs, "--docker-bin"); i_bin != -1 {
			if i_bin+1 < len(gdargs) {
				config.updateDockerBin(gdargs[i_bin+1])
			}
		}
		return args, false, config, lev
	}
	return args, false, config, lev
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

	args = append(args, []string{"-e", "ECHO_IDS=0"}...)

	args = append(args, cmds...)
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
