package main

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/urfave/cli/v3"
)

func cmdDev() *cli.Command {
	return &cli.Command{
		Name:  "dev",
		Usage: "subcommands for develop",
		Commands: []*cli.Command{
			cmdDevInit(),
			cmdDevMakeImageDir(),
			cmdDevSave(),
			cmdCopyDockerfileStocks(),
			cmdDockerfile(),
		},
	}
}

var (
	DESCRIPTION_DEV_INIT = `Initialize the root directory and base images for gdocker.
	This command creates the necessary directory structure and template files
	for building base Docker images (ubuntu_a/ubuntu_x) for the specified architecture.
	It generates directories, Dockerfiles, entrypoint scripts, and Makefiles for the base images.
	The command can be run in a dry-run mode to preview actions before execution.

	Examples)
	#> gdocker dev init
	#> gdocker dev init --arch x86_64 --dry-run`
)

func ubuntuDockerfile(platform, tag, tz string) *Dockerfile {
	d := NewDockerfile(true)
	d.AddStage(Stage{})
	d.LastStage().AddInstruction(&FROM_INST{platform, "ubuntu", tag, ""})
	d.LastStage().AddInstruction(&BLANK{})
	d.LastStage().AddInstruction(&ENV_INST{[]string{"TZ"}, []string{tz}})
	d.LastStage().AddInstruction(&VOLUMNE_INST{[]string{"/data", "/config", "/share"}})
	d.LastStage().AddInstruction(&COPY_INST{
		"shell",
		[]string{},
		[]string{"docker_prompt.sh"},
		"/config/docker_prompt.sh",
	})
	d.LastStage().AddInstruction(&COPY_INST{
		"shell",
		[]string{"--chmod=777"},
		[]string{"entry_point.sh"},
		"/usr/local/bin/entrypoint.sh",
	})
	d.LastStage().AddInstruction(&COPY_INST{
		"shell",
		[]string{},
		[]string{"cache/rush"},
		"/usr/local/bin/rush",
	})
	d.LastStage().AddInstruction(&APT_INSTALL{
		"shell",
		[]string{
			"gosu", "\\",
			"zstd", "\\",
			"tzdata", "\\",
			"ca-certificates", "\\",
			"openssl", "\\",
		},
		"&&",
	})
	return &d
}

func cmdDevInit() *cli.Command {
	return &cli.Command{
		Name:               "init",
		Usage:              "setup root directory and base images",
		UsageText:          ``,
		CustomHelpTemplate: TMPL_SUBCOMMAND_HELP,
		ArgsUsage:          "[options]",
		Description:        DESCRIPTION_DEV_INIT,
		Flags: []cli.Flag{
			FLAG_DOCKER_BIN_DEFAULT,
			FLAG_DIRECTORY,
			FLAG_ARCH,
			FLAG_TIMEZONE,
			FLAG_SHOW_ABSPATH,
			FLAG_CONFIG_DEFAULT,
			FLAG_VERBOSE,
			FLAG_DRYRUN,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("dev init", getLogLevel(cmd.Int64("verbose")))
			slog.SetDefault(logger)

			config, _ := loadConfig(cmd)

			dir := config.Dir
			arch := config.DefaultArch
			var name, platform string
			switch arch {
			case "arm":
				name = "ubuntu_a"
				platform = "linux/arm64/v8"
			case "x86_64":
				name = "ubuntu_x"
				platform = "linux/amd64"
			}

			outf, outf1, outf2 := "stdout", "stdout", "stdout"

			dir1, dir2 := filepath.Join(dir, arch, name, "22.04"), filepath.Join(dir, arch, name, "20.04")
			slog.Info(fmt.Sprintf("creating root directory for %s images", platform))
			slog.Info(fmt.Sprintf(`making directories:

%s (root directory)
└── %s (architecture root directory)
    └── %s (base image)
        ├── 22.04
        └── 20.04
`, anonymizeWd(dir, config.ShowAbspath), arch, name))
			if !cmd.Bool("dry-run") {
				mkDirAll(dir1)
				mkDirAll(dir2)
			}

			// docker_prompt.sh
			outf1, outf2 = filepath.Join(dir1, "docker_prompt.sh"), filepath.Join(dir2, "docker_prompt.sh")
			slog.Info("creating shell prompt setups:")

			box := cmd.Bool("dry-run")
			NewTemplates(TMPL_UBUNTU_PROMPT, nil).writeTemplates(outf1, box)
			NewTemplates(TMPL_UBUNTU_PROMPT, nil).writeTemplates(outf2, box)

			// entrypoint.sh
			outf1, outf2 = filepath.Join(dir1, "entrypoint.sh"), filepath.Join(dir2, "entrypoint.sh")
			slog.Info("creating entrypoint scripts:")
			NewTemplates(TMPL_UBUNTU_ENTRYPOINT, nil).writeTemplates(outf1, box)
			NewTemplates(TMPL_UBUNTU_ENTRYPOINT, nil).writeTemplates(outf2, box)

			// Dockerfile
			timezone := cmd.String("timezone")
			outf1, outf2 = filepath.Join(dir1, "Dockerfile"), filepath.Join(dir2, "Dockerfile")
			slog.Info("creating Dockerfiles:")
			ubuntuDockerfile(platform, "22.04", timezone).WriteTo(outf1, box)
			ubuntuDockerfile(platform, "20.04", timezone).WriteTo(outf2, box)

			// Makefile
			outf = filepath.Join(dir, arch, name, "Makefile")
			slog.Info("creating Makefile:")

			tms := NewTemplates(
				TMPL_MAKEFILE,
				map[string]any{
					"GdockerVersion": APP_VERSION,
					"Name":           name,
					"Tags":           []string{"22.04", "20.04"},
				},
			)

			var goarch string
			switch name {
			case "ubuntu_a":
				goarch = "arm64"
			case "ubuntu_x":
				goarch = "amd64"
			}
			tms.AddTemplate(
				TEMPLATE_RESOURCE,
				map[string]any{
					"Tag":      "22.04",
					"Resource": "rush",
					"Commands": []string{
						"curl --output $(@D)/rush.tar.gz -L https://github.com/shenwei356/rush/releases/download/v0.7.0/rush_linux_" + goarch + ".tar.gz",
						"tar -xzf $(@D)/rush.tar.gz",
						"mv rush $(@D)/rush",
					},
				},
			)
			tms.AddTemplate(
				TEMPLATE_RESOURCE,
				map[string]any{
					"Tag":      "20.04",
					"Resource": "rush",
					"Commands": []string{
						"curl --output $(@D)/rush.tar.gz -L https://github.com/shenwei356/rush/releases/download/v0.7.0/rush_linux_" + goarch + ".tar.gz",
						"tar -xzf $(@D)/rush.tar.gz",
						"mv rush $(@D)/rush",
					},
				},
			)

			tms.AddTemplate(
				TEMPLATE_OLDVER,
				map[string]any{
					"Tag":       "22.04",
					"Resources": []string{"22.04/$(DIR_OUT)/rush"},
				},
			)
			tms.AddTemplate(
				TEMPLATE_OLDVER,
				map[string]any{
					"Tag":       "20.04",
					"Resources": []string{"20.04/$(DIR_OUT)/rush"},
				},
			)

			tms.writeTemplates(outf, box)

			return nil
		},
	}
}

var (
	DESCRIPTION_DEV_MKDIR = `Prepare a template for building a Docker image.
	This command creates the directory structure and Makefile needed
	to build a new Docker image for the specified architecture, image name, and tags.
	You can also specify resources and commands for each tag,
	either directly or via standard input.
	The command can be run in a dry-run mode to preview actions before execution.

	Examples)
	#> gdocker dev mkdir --name foo --tags "bar baz"
	#> # format 1  -r {Tag}:{File}:{Cmd}
	#> # define one line command to prepare a resource for image building
	#> gdocker dev mkdir --arch x86_64 --name foo --tags "bar baz" -r bar:file.tar.gz:"curl -O ..."
	#> # format 2 -r stdin
	#> # define multi lines command to prepare resources for image building
	#> gdocker dev mkdir --name foo --tags "bar baz" \
	#>     -r stdin << 'EOF'
	#> bar
	#> resource1.txt
	#> curl -O http://example.com/resource1.txt
	#> bar
	#> baz
	#> resource2.txt.gz
	#> curl -O http://example.com/resource2.txt
	#> gzip resource2.txt
	#> baz
	#> EOF
	#> `
)

func cmdDevMakeImageDir() *cli.Command {
	return &cli.Command{
		Name:               "mkdir",
		Usage:              "prepare directory and Makefile for building image",
		UsageText:          ``,
		CustomHelpTemplate: TMPL_SUBCOMMAND_HELP,
		ArgsUsage:          "[options]",
		Description:        DESCRIPTION_DEV_MKDIR,
		Flags: []cli.Flag{
			FLAG_DIRECTORY,
			FLAG_ARCH,
			FLAG_NAME,
			FLAG_BUILD_TAGS,
			FLAG_RESOURCES,
			FLAG_CONFIG_DEFAULT,
			FLAG_VERBOSE,
			FLAG_DRYRUN,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("dev mkdir", getLogLevel(cmd.Int64("verbose")))
			slog.SetDefault(logger)

			config, _ := loadConfig(cmd)
			dir := config.Dir
			arch := config.DefaultArch

			name := cmd.String("name")

			var tag_list []string
			if cmd.IsSet("tags") {
				tag_list = append(tag_list, strings.Split(cmd.String("tags"), " ")...)
			}
			if cmd.NArg() > 0 {
				tag_list = append(tag_list, cmd.Args().Slice()...)
			}
			tm := NewTemplates(TMPL_MAKEFILE, map[string]any{"Name": name, "Tags": tag_list})
			var oldvers = make([]dataMakeOldVer, len(tag_list))
			for i := range oldvers {
				oldvers[i].Tag = tag_list[i]
			}

			var dataMakeResources []dataMakeResource
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
						dataMakeResources = append(dataMakeResources, dataMakeResource{temp_tag, resource, commands})
						i := slices.Index(tag_list, temp_tag)
						oldvers[i].Resources = append(oldvers[i].Resources, filepath.Join(temp_tag, "$(DIR_OUT)", resource))
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
					dataMakeResources = append(dataMakeResources, dataMakeResource{sp[0], sp[1], []string{sp[2]}})
					i := slices.Index(tag_list, sp[0])
					oldvers[i].Resources = append(oldvers[i].Resources, filepath.Join(sp[0], "$(DIR_OUT)", sp[1]))
				}
			}

			for _, oldver := range oldvers {
				tm.AddTemplate(TEMPLATE_OLDVER, oldver)
			}

			var outf string
			if !cmd.Bool("dry-run") {
				for _, tag := range tag_list {
					mkDirAll(filepath.Join(dir, arch, name, tag))
				}
			}
			outf = filepath.Join(dir, arch, name, "Makefile")
			tm.writeTemplates(outf, cmd.Bool("dry-run"))

			slices.SortFunc(dataMakeResources, func(a, b dataMakeResource) int {
				return len(a.Commands) - len(b.Commands)
			})

			tms := NewTemplates(
				`gdocker dev mkdir --arch {{< .Arch >}} --name {{< .Name >}} --tags "{{< .Tags >}}"`,
				map[string]any{
					"Arch": arch,
					"Name": name,
					"Tags": cmd.String("tags"),
				},
			)
			first_multiple := true
			for _, d := range dataMakeResources {
				if len(d.Commands) == 1 {
					tms.AddTemplate(` \
	-r {{< .Tag >}}:{{< .Resource >}}:'{{< index .Commands 0 >}}'`, d)
				} else {
					if first_multiple {
						tms.AddTemplate(` \
	-r stdin << 'EOF'
`, nil)
						first_multiple = false
					}
					tms.AddTemplate(`{{< .Tag >}}
{{< .Resource >}}{{< range .Commands >}}
{{< . >}}{{< end >}}
{{< .Tag >}}
`, d)
				}
			}
			if !first_multiple {
				tms.AddTemplate(`EOF
`, nil)
			}
			tms.writeTemplates(filepath.Join(dir, arch, name, "reproduce.sh"), cmd.Bool("dry-run"))

			return nil
		},
	}
}

var (
	DESCRIPTION_DEV_SAVE = `Helps to save pre-existing Dockerfiles into specified directory.
	This command copies Dockerfiles from the image building directories to the
	specified directory. The image building directories will be searched recursively
	from the root directory specified by "--dir". The command can be run in a
	dry-run mode to preview actions before run. The stocked Dockerfile name will
	be "Dockerfile_ubuntu_a;22.04", when the docker image is "ubuntu_a:22.04".

	Examples)
	#> gdocker dev save --dir docker_images/arm -n`
)

func cmdDevSave() *cli.Command {
	return &cli.Command{
		Name:               "save",
		Usage:              "save Dockerfiles into directory",
		UsageText:          ``,
		CustomHelpTemplate: TMPL_SUBCOMMAND_HELP,
		ArgsUsage:          "[options]",
		Description:        DESCRIPTION_DEV_SAVE,
		Before:             setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_DIRECTORY,
			FLAG_DIRECTORY_STOCK,
			FLAG_CONFIG_DEFAULT,
			FLAG_DRYRUN,
			FLAG_VERBOSE,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("dev save", getLogLevel(cmd.Int64("verbose")))
			slog.SetDefault(logger)

			config, _ := loadConfig(cmd)
			dir := config.Dir
			stock := config.StockDir

			ibds := searchImageBuildDir(dir, "archive")

			for _, ibd := range ibds.ibds {
				// Skip copying for Dockerfile for ubuntu_* image
				if strings.HasPrefix(ibd.dirImage, "ubuntu_") {
					slog.Info(fmt.Sprintf("skipped files in '%s'", ibd.Directory()))
					continue
				}

				arch := filepath.Base(ibd.dirParent)

				// {stock}/{arm,x86_64}/reproduce.sh_{image_name}
				source := filepath.Join(ibd.Directory(), "reproduce.sh")
				dest := filepath.Join(stock, arch, fmt.Sprintf("reproduce.sh_%s", ibd.dirImage))
				if cmd.Bool("dry-run") {
					fmt.Printf("%s -> %s\n", source, dest)
				} else {
					copyFile(source, dest)
				}

				// Copy Dockerfiles to stock directory
				// {stock}/{arm,x86_64}/Dockerfile_{image_name};{tag}
				for _, tag := range ibd.dirTags {
					source := filepath.Join(ibd.Directory(), tag, "Dockerfile")
					dest := filepath.Join(stock, arch, fmt.Sprintf("Dockerfile_%s;%s", ibd.dirImage, tag))
					if cmd.Bool("dry-run") {
						fmt.Printf("%s -> %s\n", source, dest)
					} else {
						copyFile(source, dest)
					}
				}

				// {stock}/{arm,x86_64}/cache/{image_name};{tag};{file_name}
				for _, tag := range ibd.dirTags {
					source := filepath.Join(ibd.Directory(), tag, "cache")
					if !isDir(source) {
						continue
					}
					dest := filepath.Join(stock, arch, fmt.Sprintf("cache.tar_%s;%s", ibd.dirImage, tag))
					if cmd.Bool("dry-run") {
						fmt.Printf("%s -> %s\n", source, dest)
					} else {
						createTarFile(source, dest)
						//copyFile(source, dest)
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
			FLAG_CONFIG_DEFAULT,
			FLAG_VERBOSE,
			FLAG_DRYRUN,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("dev cp", getLogLevel(cmd.Int64("verbose")))
			slog.SetDefault(logger)

			config, _ := loadConfig(cmd)
			dir := config.Dir
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

var (
	FLAG_DF_INST = &cli.StringSliceFlag{
		Name: "inst",
		Usage: `specify instruction
type        | (upper) short
            | (lower) full  { (): choice, []: optional, ...: plural }
 FROM       | from:<image>[:<tag>:<alias>]
            | FROM:<platform>:<image>:<tag>:<alias>
 RUN        | run:[<sep>]:<command>[<sep><command>...][:<op>]
            | RUN:<form>:[<sep>]:<command>[<sep><command>...][:<op>]
 ENTRYPOINT | ep:[<sep>]:<command>[<sep><command>...]
            | ENTRYPOINT:<form>:[<sep>]:<command>[<sep><command>...]
 COPY       | cp:[<sep>]:<src>[<sep><src>...]:<dest>[:<option>,<option>]
            | COPY:<form>:<sep>:<option...>:<src...>:<dest>
 WORKDIR    | wd:<directory>
            | WORKDIR:<directory>
 ENV        | env:<key=value>[:<key=value>...]
            | ENV:<key=value>[:<key=value>...]
 VOLUME     | vol:<volume>[:<volume>...]
            | VOLUME:<volume>[:<volume>...]
 LABEL      | lab:<key=value>[:<key=value>...]
            | LABEL:<key=value>[:<key=value>...]
other
 apt install | apt:<pkg>[,<pkg>...]
 blank line  | bl:
 comment     | cm:<comment>
 switch      | switch:[<sep>]
 switchonce  | switchonce:[<sep>]
`,
		Required: true,
	}
)

func cmdDockerfile() *cli.Command {
	return &cli.Command{
		Name:               "dockerfile",
		Usage:              "test",
		CustomHelpTemplate: TMPL_SUBCOMMAND_HELP,
		ArgsUsage:          "[options]",
		Description: `Compose a Dockerfile from instruction strings.
	This command parses one or more --inst values and appends the corresponding instructions to a Dockerfile.
	The output path is derived from --dir, --arch, --name, and --tag as {DIR}/{ARCH}/{NAME}/{TAG}/Dockerfile.
	Use --dry-run to preview the result without writing a file. You can temporarily change the
	argument separator by using the pseudo-instructions 'switch' and 'switchonce'.

	Examples)
	# Mix full and shorthand forms
	# Change separator once (useful for the case containing colons)
	#> gdocker dev dockerfile --name foo --tag bar \
	#>   --inst from:ubuntu
	#>   --inst switchonce:# \
	#>   --inst run:#download,https://example.com/foo.exe
	#>   --inst vol:/foo:/bar`,
		Before: setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_DF_INST,
			FLAG_DIRECTORY,
			FLAG_ARCH,
			FLAG_NAME,
			FLAG_BUILD_TAG,
			FLAG_CONFIG_DEFAULT,
			FLAG_VERBOSE,
			FLAG_DRYRUN,
		},
		DisableSliceFlagSeparator: true,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("dev dockerfile", getLogLevel(cmd.Int64("verbose")))
			slog.SetDefault(logger)

			config, _ := loadConfig(cmd)
			dir := config.Dir
			arch := config.DefaultArch
			name := cmd.String("name")
			tag := cmd.String("tag")

			df := NewDockerfile(true)

			for _, v := range cmd.StringSlice("inst") {
				inst_type, rest, ok := strings.Cut(v, ":")
				if !ok {
					slog.Error(fmt.Sprintf("no instruction type found, '%s'", v))
					os.Exit(1)
				}

				// switch is not an instruction.
				// it can be used to change the global separater to parse instruction correctlly.
				// (e.g. to parse URL like https://example.com)
				switch inst_type {
				case "switch":
					if rest == "" {
						ARG_SEPARATER = ":"
					} else {
						ARG_SEPARATER = rest
					}
					continue
				case "switchonce":
					ONCE = true
					OLD_ARG_SEPARATER = ARG_SEPARATER
					if rest == "" {
						ARG_SEPARATER = ":"
					} else {
						ARG_SEPARATER = rest
					}
					continue
				}

				// assign instruction
				var inst InstructionBuilder
				short := false
				switch inst_type {
				// use full parser
				case "FROM":
					inst = &FROM_INST{}
				case "RUN":
					inst = &RUN_INST{}
				case "ENTRYPOINT":
					inst = &ENTRYPOINT_INST{}
				case "COPY":
					inst = &COPY_INST{}
				case "WORKDIR":
					inst = &WORKDIR_INST{}
				case "ENV":
					inst = &ENV_INST{}
				case "VOLUME":
					inst = &VOLUMNE_INST{}
				case "LABEL":
					inst = &LABEL_INST{}
				// use short-hand parser
				case "lab":
					inst = &LABEL_INST{}
					short = true
				case "from":
					inst = &FROM_INST{}
					short = true
				case "run":
					inst = &RUN_INST{}
					short = true
				case "ep":
					inst = &ENTRYPOINT_INST{}
					short = true
				case "cp":
					inst = &COPY_INST{}
					short = true
				case "wd":
					inst = &WORKDIR_INST{}
					short = true
				case "env":
					inst = &ENV_INST{}
					short = true
				case "vol":
					inst = &VOLUMNE_INST{}
					short = true
				case "cm":
					inst = &COMMENT{}
				case "bl":
					inst = &BLANK{}
				case "apt":
					inst = &APT_INSTALL{}
				// undefined instruction
				default:
					slog.Error(fmt.Sprintf("invalid type '%s'", inst_type))
					os.Exit(1)
				}

				// parse instruction
				if short {
					inst.ParseShort(rest)
				} else {
					inst.Parse(rest)
				}
				if ONCE {
					ARG_SEPARATER = OLD_ARG_SEPARATER
					ONCE = false
				}

				// add instruction to the last stage
				df.AddInstruction(inst)
			}

			outf := filepath.Join(dir, arch, name, tag, "Dockerfile")
			outf = anonymizeWd(outf, config.ShowAbspath)
			df.WriteTo(outf, cmd.Bool("dry-run"))

			return nil
		},
	}
}
