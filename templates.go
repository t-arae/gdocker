package main

import (
	"bufio"
	"bytes"
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
			tm := NewTemplates(TMPL_MAKEFILE, dataMakeHeader{Name: name, Tags: tag_list})
			var oldvers = make([]dataMakeOldVer, len(tag_list))
			for i := range oldvers {
				oldvers[i].Tag = tag_list[i]
			}

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
			if slices.Contains(cmd.StringSlice("resource"), "stdin") {
				sc := bufio.NewScanner(os.Stdin)
				for sc.Scan() {
					if slices.Contains(tag_list, sc.Text()) {
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
						tm.AddTemplate(TEMPLATE_RESOURCE, dataMakeResource{temp_tag, resource, commands})
						i := slices.Index(tag_list, temp_tag)
						oldvers[i].Resources = append(oldvers[i].Resources, resource)
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
					if !slices.Contains(tag_list, sp[0]) {
						slog.Error(fmt.Sprintf("could not found tag '%s'", sp[0]))
						os.Exit(1)
					}
					tm.AddTemplate(TEMPLATE_RESOURCE, dataMakeResource{sp[0], sp[1], []string{sp[2]}})
					i := slices.Index(tag_list, sp[0])
					oldvers[i].Resources = append(oldvers[i].Resources, filepath.Join(sp[0], "$(DIR_OUT)", sp[1]))
				}
			}

			for _, oldver := range oldvers {
				tm.AddTemplate(TEMPLATE_OLDVER, oldver)
			}

			var outf string
			if !cmd.Bool("dry-run") {
				mkDirAll(filepath.Join(dir, arch, name))
			}
			outf = filepath.Join(dir, arch, name, "Makefile")
			tm.writeTemplates(outf, cmd.Bool("dry-run"))
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
		Name:   "init",
		Usage:  "setup root directory and base images",
		Before: setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_DIRECTORY,
			FLAG_ARCH,
			FLAG_DRYRUN,
			FLAG_VERBOSE,
			FLAG_TIMEZONE,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("dev init", getLogLevel(cmd.Int("verbose")))
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

			outf, outf1, outf2 := "stdout", "stdout", "stdout"

			dir1 := filepath.Join(dir, arch, name, "22.04")
			dir2 := filepath.Join(dir, arch, name, "20.04")
			slog.Info(fmt.Sprintf("creating root directory for %s images", platform))
			slog.Info(fmt.Sprintf(`making directories:

%s (root directory)
└── %s (architecture root directory)
    └── %s (base image)
        ├── 22.04
        └── 20.04
`, dir, arch, name))
			if !cmd.Bool("dry-run") {
				mkDirAll(dir1)
				mkDirAll(dir2)
			}

			// docker_prompt.sh
			outf1 = filepath.Join(dir1, "docker_prompt.sh")
			outf2 = filepath.Join(dir2, "docker_prompt.sh")
			slog.Info("creating shell prompt setups:")

			box := cmd.Bool("dry-run")
			NewTemplates(TMPL_UBUNTU_PROMPT, nil).writeTemplates(outf1, box)
			NewTemplates(TMPL_UBUNTU_PROMPT, nil).writeTemplates(outf2, box)

			// entrypoint.sh
			outf1 = filepath.Join(dir1, "entrypoint.sh")
			outf2 = filepath.Join(dir2, "entrypoint.sh")
			slog.Info("creating entrypoint scripts:")
			NewTemplates(TMPL_UBUNTU_ENTRYPOINT, nil).writeTemplates(outf1, box)
			NewTemplates(TMPL_UBUNTU_ENTRYPOINT, nil).writeTemplates(outf2, box)

			// Dockerfile
			type tmplData struct {
				RootDir  string
				Name     string
				Tag      string
				Platform string
				TimeZone string
			}

			timezone := cmd.String("timezone")
			outf1 = filepath.Join(dir1, "Dockerfile")
			outf2 = filepath.Join(dir2, "Dockerfile")
			slog.Info("creating Dockerfiles:")
			NewTemplates(
				TMPL_UBUNTU_DOCKERFILE,
				tmplData{Tag: "22.04", Platform: platform, TimeZone: timezone},
			).writeTemplates(outf1, box)
			NewTemplates(
				TMPL_UBUNTU_DOCKERFILE,
				tmplData{Tag: "20.04", Platform: platform, TimeZone: timezone},
			).writeTemplates(outf2, box)

			// Makefile
			outf = filepath.Join(dir, arch, name, "Makefile")
			slog.Info("creating Makefile:")

			tms := NewTemplates(TMPL_MAKEFILE, dataMakeHeader{Name: name, Tags: []string{"22.04", "20.04"}})

			tms.AddTemplate(TEMPLATE_RESOURCE, dataMakeResource{Tag: "22.04"})
			tms.AddTemplate(TEMPLATE_RESOURCE, dataMakeResource{Tag: "20.04"})

			tms.AddTemplate(TEMPLATE_OLDVER, dataMakeOldVer{Tag: "22.04"})
			tms.AddTemplate(TEMPLATE_OLDVER, dataMakeOldVer{Tag: "20.04"})

			tms.writeTemplates(outf, box)

			return nil
		},
	}
}

type dataMakeHeader struct {
	Name string
	Tags []string
}

type dataMakeResource = struct {
	Tag      string
	Resource string
	Commands []string
}

type dataMakeOldVer = struct {
	Tag       string
	Resources []string
}

type BoxedWriter struct {
	Title string
	Out   io.Writer
}

func (bw *BoxedWriter) Write(p []byte) (n int, err error) {
	content := strings.ReplaceAll(string(p), "\t", "   ") // Replace tabs with spaces
	lines := strings.Split(content, "\n")

	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	dashLen := maxLen - len(bw.Title) - 4
	if dashLen < 0 {
		dashLen = 0
	}
	titleLine := fmt.Sprintf("+-- %s %s+", bw.Title, strings.Repeat("-", dashLen))

	var boxedLines []string
	for _, line := range lines {
		padding := strings.Repeat(" ", maxLen-len(line))
		boxedLines = append(boxedLines, fmt.Sprintf("|%s%s|", line, padding))
	}

	bottomLine := "+" + strings.Repeat("-", maxLen) + "+"

	boxed := append([]string{titleLine}, boxedLines...)
	boxed = append(boxed, bottomLine)
	full := "\n" + strings.Join(boxed, "\n") + "\n"

	_, err = io.WriteString(bw.Out, full)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// appendBuffer appends the rendered template to the provided buffer.
func appendBuffer(buf *bytes.Buffer, tmpl string, data any) {
	parsed, err := template.New("").Delims("{{<", ">}}").Parse(tmpl)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	err = parsed.Execute(buf, data)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

type Templates struct {
	templates []string
	tmplsdata []any
}

func NewTemplates(t string, data any) *Templates {
	return &Templates{
		templates: []string{t},
		tmplsdata: []any{data},
	}
}

func (ts *Templates) AddTemplate(t string, data any) {
	ts.templates = append(ts.templates, t)
	ts.tmplsdata = append(ts.tmplsdata, data)
}

func (ts *Templates) writeTemplates(file string, box bool) {
	var err error
	var buf bytes.Buffer

	for i, t := range ts.templates {
		data := ts.tmplsdata[i]
		if data == nil {
			data = struct{}{} // Use an empty struct if no data is provided
		}
		appendBuffer(&buf, t, data)
	}

	var w io.Writer
	if file == "stdout" {
		w = os.Stdout
	} else {
		if box {
			w = &BoxedWriter{Title: file, Out: os.Stdout}
		} else {
			var f *os.File
			f, err = os.Create(file)
			if err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
			defer f.Close()
			w = f
		}
	}
	_, err = w.Write(buf.Bytes())
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
