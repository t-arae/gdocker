

# gdocker

`gdocker` is a command-line tool written in Go for managing and building
Docker images. It provides commands to show image dependencies, build
images with dependency resolution, run containers with preset options,
and manage image directories.

## Install

Before installing `gdocker`, ensure the following are installed on your
system:

- Go
- make
- docker

To install `gdocker`, run the following command in terminal:

``` bash
go install -trimpath -ldflags='-s -w' github.com/t-arae/gdocker@latest
```

If the `$GOPATH/bin` is not in the `$PATH`, add it.

``` bash
# Add this line to your ~/.profile
# Before adding, check your GOPATH by `go env` and replace below with it.
export PATH=$PATH:$GOPATH/bin
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
       0.0.4 (Docker version 28.3.2, build 578ccf6)

    COMMANDS:
       showdeps  show docker image dependencies as mermaid flowchart
       build     build docker image from list
       clean     clean docker image from list
       images    show built images with some info
       run       docker run with uid and gid
       wdrun     docker run with uid, gid and working directory
       tag       tag/untag images with specified project tag
       config    manage configuration file
       dev       subcommands for develop
       help, h   Shows a list of commands or help for one command

    GLOBAL OPTIONS:
       --verbose value, -V value          set verbosity (0-2) (default: 1)
       --docker-bin value                 path to the docker binary
       --config value [ --config value ]  configuration file
                                           (default: `$HOME/Library/Application Support/gdocker/gdocker_conf.json`, `./gdocker_conf.json`)
       --help, -h                         show help
       --version, -v                      print the version

This will display a list of all supported subcommands and their
descriptions. For help with a specific subcommand, use:

``` bash
gdocker help <subcommand>
```

Replace <subcommand> with the name of the command you want help with
(e.g., build, showdeps, run).
