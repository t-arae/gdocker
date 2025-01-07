

# gdocker

## Install

``` bash
git clone --depth 1 t-arae/gdocker
cd gdocker/
go install 
```

## main commmand

``` bash
gdocker --help
```

    NAME:
       gdocker - personal docker wrapper tool written in Go

    USAGE:
       gdocker [global options] [command [command options]]

    VERSION:
       0.0.1 (Docker version 27.4.0, build bde2b89)


    COMMANDS:
       showdeps  show docker image dependencies as mermaid flowchart
       build     build docker image from list
       run       docker run with uid and gid
       wdrun     
       images    show built images with some info
       template  prepare template for building image
       clean     clean docker image from list
       help, h   Shows a list of commands or help for one command

    GLOBAL OPTIONS:
       --verbose value, -V value  set verbosity (0-2) (default: 1)
       --docker-bin value         path to the docker binary (default: "docker")
       --help, -h                 show help
       --version, -v              print the version

## Sub command

### `gdocker showdeps`

``` bash
gdocker help showdeps
```

    NAME:
       gdocker showdeps - show docker image dependencies as mermaid flowchart

    USAGE:
       gdocker showdeps

    OPTIONS:
       --dir DIR, -d DIR          path to the root directory (DIR) for build images
       --gfm, -m                  print for GitHub Fravored Markdown (default: false)
       --verbose value, -V value  set verbosity (0-2) (default: 1)
       --help, -h                 show help

``` bash
gdocker showdeps -m -d tests/images/
```

``` mermaid
flowchart TD
    ubuntu1_a:20.04("ubuntu1_a:20.04") --> ubuntu:20.04("ubuntu:20.04")
    ubuntu1_a:22.04("ubuntu1_a:22.04") --> ubuntu:22.04("ubuntu:22.04")
    ubuntu1_a:latest("ubuntu1_a:latest") --> ubuntu1_a:22.04("ubuntu1_a:22.04")
    ubuntu2_a:20.04("ubuntu2_a:20.04") --> ubuntu1_a:20.04("ubuntu1_a:20.04")
    ubuntu2_a:22.04("ubuntu2_a:22.04") --> ubuntu1_a:22.04("ubuntu1_a:22.04")
    ubuntu2_a:latest("ubuntu2_a:latest") --> ubuntu2_a:22.04("ubuntu2_a:22.04")
```

### `gdocker build`

``` bash
gdocker help build
```

    NAME:
       gdocker build - build docker image from list

    USAGE:
       gdocker build

    OPTIONS:
       --dir DIR, -d DIR          path to the root directory (DIR) for build images
       --list FILE, -l FILE       read image names to build from FILE
       --flag STR, -f STR         a string (STR) for set to build flags
       --tag TAG, -t TAG          a string (TAG) to set image tag (default: "latest")
       --dry-run, -n              dry run (default: false)
       --verbose value, -V value  set verbosity (0-2) (default: 1)
       --docker-bin value         path to the docker binary (default: "docker")
       --help, -h                 show help

``` bash
gdocker build -d tests/ ubuntu1_a 2>/dev/null
```

    flowchart TD
        ubuntu1_a:22.04("ubuntu1_a:22.04") --> ubuntu:22.04[["ubuntu:22.04 [root]"]]
        ubuntu1_a:latest("ubuntu1_a:latest") --> ubuntu1_a:22.04("ubuntu1_a:22.04")
    make -C tests/images/arm/ubuntu1_a cache/22.04.log
    mkdir -p cache
    docker build  -t ubuntu1_a:22.04 /Users/t_arae/Dropbox/ngs_analysis/go/gdocker/tests/images/arm/ubuntu1_a//22.04/
    touch cache/22.04.log
    make -C tests/images/arm/ubuntu1_a latest
    docker tag ubuntu1_a:22.04 ubuntu1_a:latest

### `gdocker run`, `gdocker wdrun`

``` bash
gdocker help run
gdocker help wdrun
```

    NAME:
      gdocker run - docker run with uid and gid

    USAGE:
      gdocker run [command options --- ][arguments...]

    OPTIONS:
      --docker-bin value         path to the docker binary (default: "docker")
      --verbose value, -V value  set verbosity (0-2) (default: 1)

    NAME:
      gdocker wdrun

    USAGE:
      gdocker wdrun [command options --- ][arguments...]

    OPTIONS:
      --docker-bin value         path to the docker binary (default: "docker")
      --verbose value, -V value  set verbosity (0-2) (default: 1)

``` bash
gdocker run ubuntu1_a bash -c 'echo "Hello `uname`"'
```

    Hello Linux

``` bash
gdocker wdrun --verbose 0 --- ubuntu1_a ls | head -4
```

    [2025-01-07 17:39:07] wdrun [ INFO] command is 'docker run --rm -v /Users/t_arae/Dropbox/ngs_analysis/go/gdocker:/data -e LOCAL_UID=501 -e LOCAL_GID=20 -e ECHO_IDS=0 ubuntu1_a ls'
    Makefile
    README.md
    README.qmd

### `gdocker images`

``` bash
gdocker help images
```

    NAME:
       gdocker images - show built images with some info

    USAGE:
       gdocker images

    OPTIONS:
       --dir DIR, -d DIR          path to the root directory (DIR) for build images
       --built-only, -b           show only images already built (default: false)
       --exist-only, -e           show only images with building directory (default: false)
       --verbose value, -V value  set verbosity (0-2) (default: 1)
       --docker-bin value         path to the docker binary (default: "docker")
       --help, -h                 show help

``` bash
gdocker images -e -d tests/images | csvtk pretty -t
```

    ImageName          Built   Exist   BuildDir                        
    ----------------   -----   -----   --------------------------------
    ubuntu1_a:22.04    true    true    tests/images/arm/ubuntu1_a/22.04
    ubuntu1_a:latest   true    true    tests/images/arm/ubuntu1_a      
    ubuntu1_a:20.04    false   true    tests/images/arm/ubuntu1_a/20.04
    ubuntu2_a:20.04    false   true    tests/images/arm/ubuntu2_a/20.04
    ubuntu2_a:22.04    false   true    tests/images/arm/ubuntu2_a/22.04
    ubuntu2_a:latest   false   true    tests/images/arm/ubuntu2_a      

### `gdocker clean`

``` bash
gdocker help clean
```

    NAME:
       gdocker clean - clean docker image from list

    USAGE:
       gdocker clean

    OPTIONS:
       --dir DIR, -d DIR          path to the root directory (DIR) for build images
       --list FILE, -l FILE       read image names to build from FILE
       --all, -a                  select all (default: false)
       --dry-run, -n              dry run (default: false)
       --verbose value, -V value  set verbosity (0-2) (default: 1)
       --help, -h                 show help

### `gdocker template`

``` bash
gdocker help template
```

    NAME:
       gdocker template - prepare template for building image

    USAGE:
       gdocker template

    OPTIONS:
       --dir DIR, -d DIR                                          path to the root directory (DIR) for build images
       --name value, -n value                                     image name
       --tags value, -t value                                     image tags
       --resource value, -r value [ --resource value, -r value ]  resource
       --prompt                                                   show prompt (default: false)
       --entry                                                    show entrypoint (default: false)
       --docker                                                   show dockerfile (default: false)
       --verbose value, -V value                                  set verbosity (0-2) (default: 1)
       --help, -h                                                 show help

## TODOs

- global
  - [x] set log level
  - [x] include docker version
- `showdeps`
  - [x] show dependency graph as mermeid format
  - [ ] run with selected images
- `build`
  - [x] detect dependency
  - [x] solve dependencies
  - [x] visualize dependencies
  - [x] export build instructions
  - [ ] enable build flags
- `run` `wdrun`
  - [x] preset uid/gid/working directory
  - [ ] help message
- `images`
  - [x] show/check image directory
  - [ ] show images from directory
- `template`
  - [x] prepare template
  - [ ] choose output (stdout or file)
  - [ ] check directory
  - [ ] make directory tree
- `config`
  - [ ] prepare a config file for `gdocker`
  - [ ] set image directory

## Requirements

- Go
- make
