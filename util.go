package main

import (
	"bufio"
	"cmp"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func DirLs(dir string) []string {
	files, err := os.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	var paths []string
	for _, file := range files {
		if file.IsDir() {
			if file.Name() == "archive" {
				continue
			}
			paths = append(paths, DirLs(filepath.Join(dir, file.Name()))...)
			continue
		}
		paths = append(paths, filepath.Join(dir, file.Name()))
	}
	return paths
}

func findDockerfile(dir string) []string {
	files := DirLs(dir)
	var dockerfiles []string
	for _, file := range files {
		if filepath.Base(file) == "Dockerfile" {
			dockerfiles = append(dockerfiles, file)
		}
	}
	return dockerfiles
}

func findLines(path string, prefix string) []string {
	var results []string

	f, err := os.Open(path)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	defer f.Close()
	s := bufio.NewScanner(f)

	for s.Scan() {
		if strings.HasPrefix(s.Text(), prefix) {
			results = append(results, s.Text())
		}
	}

	if err = s.Err(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	return results
}

// Dockerfileを読み込んで、FROM命令から依存しているDockerImageを返す
func readDockerfile(dfile string) []DockerImage {
	from_directives := findLines(dfile, "FROM")
	var deps []DockerImage
	for _, v := range from_directives {
		li := strings.Split(strings.TrimPrefix(v, "FROM "), " ")
		if strings.HasPrefix(li[0], "--platform=") {
			deps = append(deps, NewDockerImage(li[1]))
		} else {
			deps = append(deps, NewDockerImage(li[0]))
		}
	}
	return deps
}

// Makefileを読み込んで、latest tag依存しているDockerImageを返す
func readMakefile(mfile string) DockerImage {
	img_name := strings.TrimPrefix(findLines(mfile, "IMG_NAME = ")[0], "IMG_NAME = ")
	latest_ver := strings.TrimPrefix(findLines(mfile, "LATEST_VERSION = ")[0], "LATEST_VERSION = ")
	return NewDockerImage(fmt.Sprintf("%s:%s", img_name, latest_ver))
}

// Dockerfileのパスのスライスを受け取って、Dependencyのスライスを返す
func findDependencyFromDockerfiles(dfiles []string) []Dependency {
	var deps []Dependency
	for _, dfile := range dfiles {
		dd := filepath.Dir(dfile)
		dddd := filepath.Dir(dd)

		// Dockerfileから読み取った依存関係を追加
		left := NewDockerImage(fmt.Sprintf("%s:%s", filepath.Base(dddd), filepath.Base(dd)))
		for _, right := range readDockerfile(dfile) {
			if left.String() == right.String() {
				continue
			}
			deps = append(deps, Dependency{left, right})
		}

		// latest tagの依存関係を追加
		left = NewDockerImage(fmt.Sprintf("%s:%s", filepath.Base(dddd), "latest"))
		right := readMakefile(filepath.Join(dddd, "Makefile"))
		if left.String() == right.String() {
			continue
		}
		deps = append(deps, Dependency{left, right})
	}

	// 重複した依存関係を除く
	slices.SortFunc(deps, func(a, b Dependency) int {
		if n := cmp.Compare(a.From.String(), b.From.String()); n != 0 {
			return n
		}
		return cmp.Compare(a.To.String(), b.To.String())
	})
	deps = slices.Compact(deps)

	return deps
}
