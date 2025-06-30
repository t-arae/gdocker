package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/urfave/cli/v3"
)

func cmdDev() *cli.Command {
	return &cli.Command{
		Name:  "dev",
		Usage: "subcommands for develop",
		Commands: []*cli.Command{
			cmdMakeRootDir(),
			cmdMakeImageDir(),
			cmdCopyDockerfileStocks(),
			cmdSaveDockerfileStocks(),
		},
	}
}

func cmdSaveDockerfileStocks() *cli.Command {
	return &cli.Command{
		Name:               "save",
		Usage:              "save Dockerfiles into directory",
		UsageText:          ``,
		CustomHelpTemplate: TMPL_SUBCOMMAND_HELP,
		ArgsUsage:          "[options]",
		Description: `Helps to save pre-existing Dockerfiles into specified directory.
This command copies Dockerfiles from the image building directories to the
specified directory. The image building directories will be searched recursively
from the root directory specified by "--dir". The command can be run in a
dry-run mode to preview actions before run. The stocked Dockerfile name will
be "Dockerfile_ubuntu_a;22.04", when the docker image is "ubuntu_a:22.04".

Examples)
#> gdocker dev save --dir docker_images/arm --stock stock/ -n`,
		Before: setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_DIRECTORY,
			FLAG_DIRECTORY_STOCK,
			FLAG_DRYRUN,
			FLAG_VERBOSE,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("dev save", getLogLevel(cmd.Int("verbose")))
			slog.SetDefault(logger)

			dir := cmd.String("dir")
			stock := cmd.String("stock")

			ibds := searchImageBuildDir(dir, "archive")

			for _, ibd := range ibds.ibds {
				// Skip copying for Dockerfile for ubuntu_* image
				if strings.HasPrefix(ibd.dirImage, "ubuntu_") {
					slog.Info(fmt.Sprintf("skipped files in '%s'", ibd.Directory()))
					continue
				}
				for _, tag := range ibd.dirTags {
					source := filepath.Join(ibd.Directory(), tag, "Dockerfile")
					dest := filepath.Join(stock, filepath.Base(ibd.dirParent), fmt.Sprintf("Dockerfile_%s;%s", ibd.dirImage, tag))
					if cmd.Bool("dry-run") {
						fmt.Printf("%s -> %s\n", source, dest)
					} else {
						copyFile(source, dest)
					}
				}
			}
			return nil
		},
	}
}

func cmdCopyDockerfileStocks() *cli.Command {
	return &cli.Command{
		Name:               "cp",
		Usage:              "copy Dockerfile",
		CustomHelpTemplate: TMPL_SUBCOMMAND_HELP,
		ArgsUsage:          "[options]",
		Description: `Helps to copy stocked Dockerfiles into image building directories.
This command copies Dockerfiles from the stock directory to the appropriate
image building directories. The 'correct' image building directory will be
searched from the root directory specified by "--dir". The command can be
run in a dry-run mode to preview actions before run.

Examples)
#> gdocker dev cp --dir docker_images/ --stock stock/ -n`,
		Before: setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_DIRECTORY,
			FLAG_DIRECTORY_STOCK,
			FLAG_DRYRUN,
			FLAG_VERBOSE,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("dev cp", getLogLevel(cmd.Int("verbose")))
			slog.SetDefault(logger)

			dir := cmd.String("dir")
			stock := cmd.String("stock")

			type Stock struct {
				path  string
				arch  string
				iname string
				tag   string
			}
			var sts []Stock

			skip_func := func(path string, d fs.DirEntry, err error) error {
				if !d.IsDir() {
					if v, ok := strings.CutPrefix(filepath.Base(path), "Dockerfile_"); ok && strings.Contains(v, ";") {
						temp := strings.SplitN(v, ";", 2)
						arch := filepath.Base(filepath.Dir(path))
						if slices.Index([]string{"arm", "x86_64"}, arch) == -1 {
							slog.Error(fmt.Sprintf("invalid directory. '%s'", path))
							return nil
						}
						sts = append(sts, Stock{path, arch, temp[0], temp[1]})
					}
				}
				return nil
			}
			if err := filepath.WalkDir(stock, skip_func); err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}

			for _, st := range sts {
				outf := filepath.Join(dir, st.arch, st.iname, st.tag, "Dockerfile")
				if !isDir(filepath.Dir(outf)) {
					continue
				}
				fmt.Printf("%s -> %s\n", st.path, outf)
				if !cmd.Bool("dry-run") {
					copyFile(st.path, outf)
				}
			}

			return nil
		},
	}
}

type Resource struct {
	tagNum       int
	ResourceName string
}
type tmplDataMakefile struct {
	RootDir   string
	Name      string
	Tags      []string
	Map       map[string]int
	Resources []Resource
	Commands  [][]string
}

func NewTmplDataMakefile(root string, name string, tags []string) tmplDataMakefile {
	t := tmplDataMakefile{
		RootDir: root,
		Name:    name,
		Tags:    tags,
	}
	t.Map = make(map[string]int)
	for idx, tag := range t.Tags {
		t.Map[tag] = idx
	}
	return t
}

func (t *tmplDataMakefile) addResource(tag string, rname string, rcommand []string) {
	idx := t.Map[tag]
	t.Resources = append(t.Resources, Resource{idx, rname})
	t.Commands = append(t.Commands, rcommand)
}

func (t *tmplDataMakefile) writeResourceTemplate(tag string, file string, append bool) {
	var tmplData struct {
		Tag      string
		Resource string
		Commands []string
	}
	i := t.Map[tag]
	tmplData.Tag = tag
	for j, r := range t.Resources {
		if r.tagNum == i {
			tmplData.Resource = r.ResourceName
			tmplData.Commands = t.Commands[j]
			writeTemplate(TEMPLATE_RESOURCE, tmplData, file, append)
		}
	}
}

func (t *tmplDataMakefile) writeImageTemplate(tag string, file string, Append bool) {
	var tmplData struct {
		Tag       string
		Resources []string
	}
	i := t.Map[tag]
	if i == 0 {
		tmplData.Tag = "$(LATEST_VERSION)"
	} else {
		tmplData.Tag = tag
	}
	for _, r := range t.Resources {
		if r.tagNum == i {
			tmplData.Resources = append(tmplData.Resources, strings.Join([]string{tag, "/$(DIR_OUT)/", r.ResourceName}, ""))
		}
	}
	writeTemplate(TEMPLATE_OLDVER, tmplData, file, Append)
}

func cmdMakeImageDir() *cli.Command {
	return &cli.Command{
		Name:   "mkdir",
		Usage:  "prepare template for building image",
		Before: setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_DIRECTORY,
			FLAG_ARCH,
			FLAG_NAME,
			FLAG_TAGS,
			FLAG_RESOURCES,
			FLAG_DRYRUN,
			FLAG_VERBOSE,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("dev mkdir", getLogLevel(cmd.Int("verbose")))
			slog.SetDefault(logger)

			dir := cmd.String("dir")
			arch := cmd.String("arch")
			name := cmd.String("name")

			var tag_list []string
			if cmd.IsSet("tags") {
				tag_list = append(tag_list, strings.Split(cmd.String("tags"), " ")...)
			}
			if cmd.NArg() > 0 {
				tag_list = append(tag_list, cmd.Args().Slice()...)
			}
			tm := NewTmplDataMakefile(dir, name, tag_list)

			// format `-r {Tag}:{File}:{Cmd}` or `-r stdin`
			// ex) `-r 22.04:software.tar.gz:"curl -O https://example.com/software.tar.gz"`
			//  or
			// ex) `-r stdin << 'EOF'
			// ex) 22.04
			// ex) software.tar.gz:curl -O https://example.com/software.tar.gz
			// ex) gzip -xzf software.tar.gz
			// ex) ./software \
			// ex)     -o example.txt
			// ex) 22.04
			// ex) EOF
			// ex) `
			if slices.Index(cmd.StringSlice("resource"), "stdin") != -1 {
				sc := bufio.NewScanner(os.Stdin)
				for sc.Scan() {
					if _, exist := tm.Map[sc.Text()]; exist {
						temp_tag := sc.Text()
						if !sc.Scan() {
							slog.Error(fmt.Sprintf("insufficient resource '%s'", temp_tag))
							os.Exit(1)
						}
						resource := sc.Text()
						var commands []string
						for sc.Scan() {
							if sc.Text() == temp_tag {
								break
							}
							commands = append(commands, sc.Text())
						}
						tm.addResource(temp_tag, resource, commands)
					} else {
						slog.Error("invalid stdin")
						os.Exit(1)
					}
				}
			}
			if cmd.IsSet("resource") {
				for _, r := range cmd.StringSlice("resource") {
					if r == "stdin" {
						continue
					}
					sp := strings.SplitN(r, ":", 3)
					if len(sp) != 3 {
						slog.Error(fmt.Sprintf("%s does not contain ':'", r))
						os.Exit(1)
					}

					if _, ok := tm.Map[sp[0]]; !ok {
						slog.Error(fmt.Sprintf("could not found tag '%s'", sp[0]))
						os.Exit(1)
					}
					tm.addResource(sp[0], sp[1], []string{sp[2]})
				}
			}

			var outf string
			if cmd.Bool("dry-run") {
				outf = "stdout"
			} else {
				mkDirAll(filepath.Join(dir, arch, name))
				outf = filepath.Join(dir, arch, name, "Makefile")
			}
			writeTemplate(TMPL_MAKEFILE, tm, outf, false)
			for _, tag := range tm.Tags {
				if !cmd.Bool("dry-run") {
					mkDirAll(filepath.Join(dir, arch, name, tag))
				}
				tm.writeResourceTemplate(tag, outf, outf != "stdout")
				tm.writeImageTemplate(tag, outf, outf != "stdout")
			}

			return nil
		},
	}
}

func mkDirAll(dir string) {
	if err := os.MkdirAll(dir, 0777); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func cmdMakeRootDir() *cli.Command {
	return &cli.Command{
		Name:   "mkroot",
		Usage:  "prepare template for building image",
		Before: setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_DIRECTORY,
			FLAG_ARCH,
			FLAG_DRYRUN,
			FLAG_VERBOSE,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("dev mkroot", getLogLevel(cmd.Int("verbose")))
			slog.SetDefault(logger)

			dir := cmd.String("dir")
			arch := cmd.String("arch")
			var name, platform string
			switch arch {
			case "arm":
				name = "ubuntu_a"
				platform = "linux/arm64/v8"
			case "x86_64":
				name = "ubuntu_x"
				platform = "linux/amd64"
			default:
				slog.Error("invalid arch string.")
				os.Exit(1)
			}

			type tmplData struct {
				RootDir  string
				Name     string
				Tag      string
				Platform string
			}

			outf, outf1, outf2 := "stdout", "stdout", "stdout"

			dir1 := filepath.Join(dir, arch, name, "22.04")
			dir2 := filepath.Join(dir, arch, name, "20.04")
			if !cmd.Bool("dry-run") {
				mkDirAll(dir1)
				mkDirAll(dir2)
			}

			// docker_prompt.sh
			if cmd.Bool("dry-run") {
				fmt.Printf("\n### %s and %s\n", filepath.Join(dir1, "docker_prompt.sh"), filepath.Join(dir2, "docker_prompt.sh"))
			} else {
				outf1 = filepath.Join(dir1, "docker_prompt.sh")
				outf2 = filepath.Join(dir2, "docker_prompt.sh")
				writeTemplate(TMPL_UBUNTU_PROMPT, nil, outf1, false)
			}
			writeTemplate(TMPL_UBUNTU_PROMPT, nil, outf2, false)

			// entrypoint.sh
			if cmd.Bool("dry-run") {
				fmt.Printf("\n### %s and %s\n", filepath.Join(dir1, "entrypoint.sh"), filepath.Join(dir2, "entrypoint.sh"))
			} else {
				outf1 = filepath.Join(dir1, "entrypoint.sh")
				outf2 = filepath.Join(dir2, "entrypoint.sh")
				writeTemplate(TMPL_UBUNTU_ENTRYPOINT, nil, outf1, false)
			}
			writeTemplate(TMPL_UBUNTU_ENTRYPOINT, nil, outf2, false)

			// Dockerfile
			if !cmd.Bool("dry-run") {
				outf1 = filepath.Join(dir1, "Dockerfile")
				outf2 = filepath.Join(dir2, "Dockerfile")
			}
			if cmd.Bool("dry-run") {
				fmt.Printf("\n### %s\n", outf1)
			}
			writeTemplate(TMPL_UBUNTU_DOCKERFILE, tmplData{Tag: "22.04", Platform: platform}, outf1, false)
			if cmd.Bool("dry-run") {
				fmt.Printf("\n### %s\n", outf2)
			}
			writeTemplate(TMPL_UBUNTU_DOCKERFILE, tmplData{Tag: "20.04", Platform: platform}, outf2, false)

			// Makefile
			if cmd.Bool("dry-run") {
				fmt.Printf("\n### %s\n", filepath.Join(dir, arch, name, "Makefile"))
			} else {
				outf = filepath.Join(dir, arch, name, "Makefile")
			}

			tm := NewTmplDataMakefile(dir, name, []string{"22.04", "20.04"})
			writeTemplate(TMPL_MAKEFILE, tm, outf, false)
			tm.writeResourceTemplate("22.04", outf, outf != "stdout")
			tm.writeResourceTemplate("20.04", outf, outf != "stdout")
			tm.writeImageTemplate("22.04", outf, outf != "stdout")
			tm.writeImageTemplate("20.04", outf, outf != "stdout")

			return nil
		},
	}
}

func writeTemplate(t string, data any, file string, append bool) {
	var err error
	var w io.Writer
	if file == "stdout" {
		w = os.Stdout
	} else {
		var f *os.File
		if append {
			f, err = os.OpenFile(file, os.O_WRONLY|os.O_APPEND, 0666)
		} else {
			f, err = os.Create(file)
		}
		if err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}

	tmpl, err := template.New("").Delims("{{<", ">}}").Parse(t)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
