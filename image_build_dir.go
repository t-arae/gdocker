package main

import (
	"cmp"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type ImageBuildDir struct {
	dirParent string
	dirImage  string
	dirTags   []string
	tagLatest int
}

// 指定したディレクトリからImageBuildDirを探索して返す
func searchImageBuildDir(path string) []ImageBuildDir {
	var ibds []ImageBuildDir

	skipDirFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			err = fmt.Errorf("skipDirFunc Error:%v", err)
			return err
		}

		if !d.IsDir() {
			return nil
		}

		if filepath.Base(path) == "archive" {
			return filepath.SkipDir
		}

		ibd, ok := NewImageBuildDir(filepath.Dir(path), filepath.Base(path))
		if ok {
			ibds = append(ibds, ibd)
			return filepath.SkipDir
		}
		return nil
	}

	err := filepath.WalkDir(path, skipDirFunc)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	return ibds
}

func NewImageBuildDir(parent string, image string) (ImageBuildDir, bool) {
	var ibd ImageBuildDir
	ibd.dirParent = parent
	ibd.dirImage = image

	image_path := filepath.Join(parent, image)

	// Check Makefile
	if isDir(image_path) {
		if !isFile(filepath.Join(image_path, "Makefile")) {
			return ibd, false
		}
	} else {
		return ibd, false
	}

	// Search tags and check Dockerfile
	err := filepath.WalkDir(image_path, func(path string, d fs.DirEntry, err error) error {
		// ファイルと直下のディレクトリ以外の場合はスキップ
		if isFile(path) || filepath.Dir(path) != image_path {
			return nil
		}
		// Dockerfileがなければスキップ
		if !isFile(filepath.Join(path, "Dockerfile")) {
			return nil
		}
		ibd.dirTags = append(ibd.dirTags, filepath.Base(path))
		return filepath.SkipDir
	})
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	if len(ibd.dirTags) == 0 {
		return ibd, false
	}

	err = ibd.findLatestImageFromMakefile()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	return ibd, true
}

func (ibd *ImageBuildDir) ImageNames() []string {
	inames := make([]string, len(ibd.dirTags))
	for _, tag := range ibd.dirTags {
		inames = append(inames, fmt.Sprintf("%s:%s", ibd.dirImage, tag))
	}
	return inames
}

type ImageBuildDirs []ImageBuildDir

func (ibds ImageBuildDirs) ImageNames() []string {
	var inames []string
	for _, ibd := range ibds {
		inames = append(inames, ibd.ImageNames()...)
	}
	return inames
}

// Dockerfileを読み込んで、FROM命令から依存しているDockerImageを返す
func (ibd *ImageBuildDir) findDependenciesFromDockerfile() []Dependency {
	var deps []Dependency
	var left, right DockerImage
	for _, tag := range ibd.dirTags {
		left = NewDockerImage(ibd.dirImage + ":" + tag)
		dfile := filepath.Join(ibd.dirParent, ibd.dirImage, tag, "Dockerfile")
		from_directives := findLines(dfile, "FROM")
		for _, v := range from_directives {
			li := strings.Split(strings.TrimPrefix(v, "FROM "), " ")
			if strings.HasPrefix(li[0], "--platform=") {
				right = NewDockerImage(li[1])
			} else {
				right = NewDockerImage(li[0])
			}
			deps = append(deps, Dependency{left, right})
		}
	}
	return deps
}

// Makefileを読み込んで、latest tag依存しているDockerImageを返す
func (ibd *ImageBuildDir) findLatestImageFromMakefile() error {
	mfile := filepath.Join(ibd.dirParent, ibd.dirImage, "Makefile")
	latest_ver := strings.TrimPrefix(findLines(mfile, "LATEST_VERSION = ")[0], "LATEST_VERSION = ")
	for i, tag := range ibd.dirTags {
		if tag == latest_ver {
			ibd.tagLatest = i
			return nil
		}
	}
	return fmt.Errorf("no latest tag found")
}

// Dockerfileのパスのスライスを受け取って、Dependencyのスライスを返す
func findDependencyFromDockerfiles(ibds []ImageBuildDir) []Dependency {
	var deps []Dependency
	for _, ibd := range ibds {
		// Dockerfileから読み取った依存関係を追加
		deps = append(deps, ibd.findDependenciesFromDockerfile()...)

		// latest tagの依存関係を追加
		err := ibd.findLatestImageFromMakefile()
		if err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}

		deps = append(deps, Dependency{
			NewDockerImage(fmt.Sprintf("%s:%s", ibd.dirImage, "latest")),
			NewDockerImage(fmt.Sprintf("%s:%s", ibd.dirImage, ibd.dirTags[ibd.tagLatest])),
		})
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
