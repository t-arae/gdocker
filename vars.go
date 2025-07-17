package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/urfave/cli/v3"
)

// Templates
const (
	TMPL_SUBCOMMAND_HELP = `NAME:
	{{template "helpNameTemplate" .}}

USAGE:
	{{if .UsageText}}{{wrap .UsageText 3}}{{else}}{{.FullName}}{{if .VisibleCommands}} [command [command options]] {{end}}{{if .ArgsUsage}} {{.ArgsUsage}}{{else}}{{if .Arguments}} [arguments...]{{end}}{{end}}{{end}}

DESCRIPTION:
	{{.Description}}{{if .VisibleCommands}}

COMMANDS:{{template "visibleCommandTemplate" .}}{{end}}{{if .VisibleFlagCategories}}

OPTIONS:{{template "visibleFlagCategoryTemplate" .}}{{else if .VisibleFlags}}

OPTIONS:{{template "visibleFlagTemplate" .}}{{end}}
`
	TEMPLATE_RESOURCE = `
{{< .Tag >}}/$(DIR_OUT)/{{< .Resource >}}:
	mkdir -p $(@D){{< range .Commands >}}
	{{< . >}}{{< end >}}
`
	TEMPLATE_OLDVER = `
$(DIR_OUT)/{{< .Tag >}}.log: $(call image_out,%) : $(DIR_OUT){{< range .Resources >}} {{< . >}}{{< end >}}
	$(DOCKER_BUILD) -t $(IMG_NAME):$* $(DIR_MAKEFILE)/$*/
	$(OUTPUT_IMAGE)
`
	TMPL_MAKEFILE = `DOCKER_BIN = docker
DOCKER_BUILD_FLAG = 
DOCKER_BUILD = $(DOCKER_BIN) build $(DOCKER_BUILD_FLAG)
OUTPUT_IMAGE = touch $@

DIR_MAKEFILE := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
DIR_OUT := cache

.PHONY: latest clean-%

define image_out
  $(addprefix $(DIR_OUT)/,$(addsuffix .log,$1))
endef

IMG_NAME = {{< .Name >}}
LATEST_VERSION = {{< index .Tags 0 >}}

$(DIR_OUT)/latest.log: $(DIR_OUT)/$(LATEST_VERSION).log
	$(DOCKER_BIN) tag $(IMG_NAME):$(LATEST_VERSION) $(IMG_NAME):latest
	touch $(DIR_OUT)/latest.log

clean-%:
	$(DOCKER_BIN) rmi $(IMG_NAME):$(*)
	rm -f $(DIR_OUT)/$(*).log

$(DIR_OUT):
	mkdir -p $@
`

	TMPL_UBUNTU_DOCKERFILE = `FROM --platform={{< .Platform >}} ubuntu:{{< .Tag >}}

ENV TZ={{< .TimeZone >}}
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get -y install gosu zstd tzdata ca-certificates openssl vim
COPY cache/gargs /usr/local/bin/gargs
COPY docker_prompt.sh /etc/profile.d/docker_prompt.sh
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh
RUN mkdir /data /config /share
VOLUME ["/data", "/config", "/share"]
WORKDIR /data

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
`

	TMPL_UBUNTU_ENTRYPOINT = `#!/bin/bash

USER_ID=${LOCAL_UID:-9001}
GROUP_ID=${LOCAL_GID:-9001}
ECHO=${ECHO_IDS:-1}

if [ $ECHO -eq 1 ] ; then
    echo "Starting with UID : $USER_ID, GID: $GROUP_ID"
else
    :
fi
useradd -u $USER_ID -o -m user
groupmod -g $GROUP_ID -o user
export HOME=/home/user
cat /etc/profile.d/docker_prompt.sh >> ${HOME}/.bashrc
exec /usr/sbin/gosu user "$@"

`

	TMPL_UBUNTU_PROMPT = `
PS1='🐳 \[\e[1m\]\# \[\e[0m\]\t @\W\n\[\e[38;5;39m\]>>>\[\e[0m\] '
`
)

var (
	// Common flags
	FLAG_DIRECTORY = &cli.StringFlag{
		Name:    "dir",
		Aliases: []string{"d"},
		Usage:   "path to the root directory for build images",
	}
	FLAG_DIRECTORY_PWD = &cli.StringFlag{
		Name:        "dir",
		Aliases:     []string{"d"},
		Usage:       "path to the root directory for build images",
		DefaultText: fmt.Sprintf("absolute(`%s`)", "./docker_images"),
		Value:       filepath.Join(getWd(), "docker_images"),
	}
	FLAG_VERBOSE = &cli.IntFlag{
		Name:     "verbose",
		Aliases:  []string{"V"},
		Value:    1,
		Usage:    "set verbosity (0-2)",
		Required: false,
	}
	FLAG_DOCKER_BIN = &cli.StringFlag{
		Name:  "docker-bin",
		Usage: "path to the docker binary",
	}
	FLAG_DOCKER_BIN_DEFAULT = &cli.StringFlag{
		Name:  "docker-bin",
		Usage: "path to the docker binary",
		Value: "docker",
	}
	FLAG_ALL = &cli.BoolFlag{
		Name:    "all",
		Aliases: []string{"a"},
		Value:   false,
		Usage:   "select all images",
	}
	FLAG_DRYRUN = &cli.BoolFlag{
		Name:    "dry-run",
		Aliases: []string{"n"},
		Value:   false,
		Usage:   "dry run",
	}

	// flag for showdeps command
	FLAG_GFM = &cli.BoolFlag{
		Name:    "gfm",
		Aliases: []string{"m"},
		Value:   false,
		Usage:   "print for GitHub Fravored Markdown",
	}

	// flags for build command
	FLAG_LIST = &cli.StringFlag{
		Name:    "list",
		Aliases: []string{"l"},
		Usage:   "read image names from `FILE`",
	}
	FLAG_MAKEFLAG = &cli.StringSliceFlag{
		Name:     "flag",
		Aliases:  []string{"f"},
		Usage:    "a string (`STR`) for setting Make variables",
		Required: false,
	}
	FLAG_TAG = &cli.StringFlag{
		Name:     "tag",
		Aliases:  []string{"t"},
		Value:    "latest",
		Usage:    "a string (`TAG`) to set image tag",
		Required: false,
	}
	FLAG_ALL_LATEST = &cli.BoolFlag{
		Name:    "all-latest",
		Aliases: []string{"al"},
		Value:   false,
		Usage:   "select all latest images",
	}

	// flags for dev sub-commands
	FLAG_NAME = &cli.StringFlag{
		Name:     "name",
		Usage:    "image name",
		Required: true,
	}
	FLAG_TAGS = &cli.StringFlag{
		Name:  "tags",
		Usage: "image tags",
	}
	FLAG_TIMEZONE = &cli.StringFlag{
		Name:  "timezone",
		Usage: "set timezone for the docker image",
		Value: "Asia/Tokyo",
	}
	FLAG_RESOURCES = &cli.StringSliceFlag{
		Name:     "resource",
		Aliases:  []string{"r"},
		Usage:    "resource",
		Required: false,
	}
	FLAG_ARCH = &cli.StringFlag{
		Name:    "arch",
		Aliases: []string{"a"},
		Usage:   "set cpu architecture ('arm' or 'x86_64')",
		Value:   getCPUArch(),
		Action: func(ctx context.Context, cmd *cli.Command, v string) error {
			if slices.Index([]string{"arm", "x86_64"}, v) == -1 {
				return fmt.Errorf("flag arch must be 'arm' or 'x86_64', not '%v'", v)
			}
			return nil
		},
	}
	FLAG_DIRECTORY_STOCK = &cli.StringFlag{
		Name:        "stock",
		Aliases:     []string{"s"},
		Usage:       "directory path for saving Dockfiles",
		DefaultText: fmt.Sprintf("absolute(`%s`)", "./stock"),
		Value:       filepath.Join(getWd(), "stock"),
	}
	FLAG_CONFIG_DEFAULT = &cli.StringSliceFlag{
		Name:        "config",
		Usage:       "configuration file\n\t",
		DefaultText: fmt.Sprintf("`{OS_CONFIG_DIR}/gdocker/%s`, `./gdocker_conf.json`", filepath.Base(getGlobalConfigFile())),
		Value:       []string{getGlobalConfigFile(), "gdocker_conf.json"},
	}
	FLAG_CONFIG_GLOBAL = &cli.StringSliceFlag{
		Name:        "config",
		Usage:       "configuration file\n\t",
		DefaultText: fmt.Sprintf("`{OS_CONFIG_DIR}/gdocker/%s`", filepath.Base(getGlobalConfigFile())),
		Value:       []string{getGlobalConfigFile()},
	}
)

func getCPUArch() string {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "amd":
		arch = "x86_64"
	case "arm64":
		arch = "arm"
	}
	return arch
}

func getGlobalConfigFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	dir = filepath.Join(dir, "gdocker")
	mkDirAll(dir)
	return filepath.Join(dir, "gdocker_conf.json")
}
