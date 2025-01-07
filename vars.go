package main

import "github.com/urfave/cli/v3"

const (
	USAGE_DIRECTORY = "path to the root directory (`DIR`) for build images"
	USAGE_LIST      = "read image names to build from `FILE`"
	USAGE_BUILDFLAG = "a string (`STR`) for set to build flags"
	USAGE_TAGS      = "a string (`TAG`) to set image tag"
)

var (
	// Common flags
	FLAG_DIRECTORY = &cli.StringFlag{
		Name:     "dir",
		Aliases:  []string{"d"},
		Usage:    USAGE_DIRECTORY,
		Required: true,
	}
	FLAG_VERBOSE = &cli.UintFlag{
		Name:     "verbose",
		Aliases:  []string{"V"},
		Value:    0,
		Usage:    "set verbosity (0-2)",
		Required: false,
	}
	FLAG_DOCKER_BIN = &cli.StringFlag{
		Name:  "docker-bin",
		Usage: "path to the docker binary",
		Value: "docker",
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
		Usage:   USAGE_LIST,
	}
	FLAG_BUILDFLAG = &cli.StringFlag{
		Name:     "flag",
		Aliases:  []string{"f"},
		Value:    "",
		Usage:    USAGE_BUILDFLAG,
		Required: false,
	}
	FLAG_TAG = &cli.StringFlag{
		Name:     "tag",
		Aliases:  []string{"t"},
		Value:    "latest",
		Usage:    USAGE_TAGS,
		Required: false,
	}

	// flags for template command
	FLAG_NAME = &cli.StringFlag{
		Name:     "name",
		Aliases:  []string{"n"},
		Usage:    "image name",
		Required: true,
	}
	FLAG_TAGS = &cli.StringFlag{
		Name:     "tags",
		Aliases:  []string{"t"},
		Usage:    "image tags",
		Required: true,
	}
	FLAG_RESOURCES = &cli.StringSliceFlag{
		Name:     "resource",
		Aliases:  []string{"r"},
		Usage:    "resource",
		Required: false,
	}
)
