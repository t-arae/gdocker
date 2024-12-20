package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/user"

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
