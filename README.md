

# gdocker

`gdocker` is a command-line tool written in Go for managing and building
Docker images, especially for bioinformatics tools. It provides commands
to show image dependencies, build images with dependency resolution, run
containers with preset options, and manage image directories. The tool
supports features like visualizing dependency graphs, exporting build
instructions, and preparing templates for new images.

## Install

Before installing `gdocker`, ensure the following are installed on your
system:

- git
- Go
- make
- docker

To install `gdocker`, run the following command in terminal:

``` bash
go install github.com/t-arae/gdocker@latest
```

## main commmand

To see the available commands and global options, run:

``` bash
gdocker --help
```

    NAME:
       gdocker - personal docker wrapper tool written in Go

    USAGE:
       gdocker [global options] [command [command options]]

    VERSION:
       0.0.2 (Docker version 28.2.2, build e6534b4)

    COMMANDS:
       showdeps  show docker image dependencies as mermaid flowchart
       build     build docker image from list
       clean     clean docker image from list
       images    show built images with some info
       run       docker run with uid and gid
       wdrun     docker run with uid, gid and working directory
       dev       subcommands for develop
       help, h   Shows a list of commands or help for one command

    GLOBAL OPTIONS:
       --verbose value, -V value          set verbosity (0-2) (default: 1)
       --docker-bin value                 path to the docker binary
       --config value [ --config value ]  configuration file
                                           (default: `{OS_CONFIG_DIR}/gdocker/gdocker_conf.json`, `./gdocker_conf.json`)
       --help, -h                         show help
       --version, -v                      print the version

This will display a list of all supported subcommands and their
descriptions. For help with a specific subcommand, use:

``` bash
gdocker help <subcommand>
```

Replace <subcommand> with the name of the command you want help with
(e.g., build, showdeps, run).
