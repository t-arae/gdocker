package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/template"

	"github.com/urfave/cli/v3"
)

func cmdTemplate() *cli.Command {
	return &cli.Command{
		Name:  "template",
		Usage: "prepare template for building image",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "dir",
				Aliases:  []string{"d"},
				Value:    "",
				Usage:    "directory",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "name",
				Aliases:  []string{"n"},
				Usage:    "image name",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "tags",
				Aliases:  []string{"t"},
				Usage:    "image tags",
				Required: true,
			},
			&cli.StringSliceFlag{
				Name:     "resource",
				Aliases:  []string{"r"},
				Usage:    "resource",
				Required: false,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("template")
			slog.SetDefault(logger)

			dir := cmd.String("d")

			name := cmd.String("name")

			var tags []string
			if cmd.IsSet("tags") {
				tags = strings.Split(cmd.String("tags"), " ")
			}

			if cmd.NArg() > 0 {
				tags = append(tags, cmd.Args().Slice()...)
			}

			type tmplData struct {
				Name    string
				Tag     string
				RootDir string
			}

			writeTemplate(TEMPLATE_MAKEFILE, tmplData{
				Name:    name,
				Tag:     tags[0],
				RootDir: dir,
			})

			if len(tags) > 1 {
				for _, tag := range tags[1:] {
					writeTemplate(TEMPLATE_OLDVER, tmplData{
						Name:    name,
						Tag:     tag,
						RootDir: dir,
					})
				}
			}

			if cmd.IsSet("resource") {
				for _, resource := range cmd.StringSlice("resource") {
					buildHoge(resource)
				}
			}

			return nil
		},
	}
}

func writeTemplate(t string, data any) {
	tmpl, err := template.New("").Delims("{{<", ">}}").Parse(t)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	err = tmpl.Execute(os.Stdout, data)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

// format {Tag}:{File}:{Cmd}
// ex) 22.04:software.tar.gz:"curl -O https://example.com/software.tar.gz"
func buildHoge(resource string) {
	type tmplData struct {
		Tag  string
		File string
		Cmd  string
	}

	sp := strings.SplitN(resource, ":", 3)
	if len(sp) != 3 {
		slog.Error(fmt.Sprintf("%s does not contain ':'", resource))
		os.Exit(1)
	}

	writeTemplate(TEMPLATE_RESOURCE, tmplData{
		Tag:  sp[0],
		File: sp[1],
		Cmd:  sp[2],
	})
}

const TEMPLATE_RESOURCE = `
SOURCE_{{< .Tag >}} = {{< .Tag >}}/$(DIR_OUT)/{{< .File >}}
$(SOURCE_{{< .Tag >}}):
	mkdir -p $(@D)
	{{< .Cmd >}}
`

const TEMPLATE_OLDVER = `
$(DIR_OUT)/{{< .Tag >}}.log: $(call image_out,%) : $(DIR_OUT)
	$(DOCKER_BUILD) -t $(IMG_NAME):$* $(DIR_MAKEFILE)/$*/
	$(OUTPUT_IMAGE)
`

const TEMPLATE_MAKEFILE = `DOCKER_BUILD_FLAG = 
DOCKER_BUILD = docker build $(DOCKER_BUILD_FLAG)
OUTPUT_IMAGE = touch $@

DIR_MAKEFILE := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
DIR_OUT := cache

.PHONY: latest clean version_short

define image_out
  $(addprefix $(DIR_OUT)/,$(addsuffix .log,$1))
endef

IMG_NAME = {{< .Name >}}
LATEST_VERSION = {{< .Tag >}}

latest: $(DIR_OUT)/$(LATEST_VERSION).log
	docker tag $(IMG_NAME):$(LATEST_VERSION) $(IMG_NAME):latest

version_short: $(DIR_OUT)/$(LATEST_VERSION).log
	@echo "$(IMG_NAME): $(LATEST_VERSION)"

clean:
	rm -rf $(DIR_OUT)/
	docker images --format "$(IMG_NAME):{{.Tag}}" $(IMG_NAME) | \
		xargs -I {} docker rmi {}

$(DIR_OUT):
	mkdir -p $@

$(DIR_OUT)/$(LATEST_VERSION).log: $(call image_out,%) : $(DIR_OUT)
	$(DOCKER_BUILD) -t $(IMG_NAME):$* $(DIR_MAKEFILE)/$*/
	$(OUTPUT_IMAGE)
`
