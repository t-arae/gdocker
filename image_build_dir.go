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

type DockerImage struct {
	Name   string // "ubuntu_a" (image name)
	Tag    string // "latest" (tag name)
	IsRoot bool
}

// DockerImageコンストラクタ
func NewDockerImage(image_name string) DockerImage {
	var d DockerImage

	// image_nameにtagが含まれない場合はlatest tagを設定する
	separated := strings.Split(image_name, ":")
	if len(separated) == 1 {
		separated = append(separated, "latest")
	}
	d.Name = separated[0]
	d.Tag = separated[1]

	return d
}

// Dockerimageの名前（例: ubuntu_a:latest）を返す
func (d DockerImage) String() string {
	if d.Name == "" {
		return ""
	}
	return d.Name + ":" + d.Tag
}

func Strings(ds []DockerImage) []string {
	names := make([]string, len(ds))
	for _, d := range ds {
		names = append(names, d.String())
	}
	return names
}

type ImageBuildDir struct {
	dirParent string
	dirImage  string
	dirTags   []string
	tagLatest int
	imgs      []DockerImage
}

// 指定したディレクトリからImageBuildDirを探索して返す
func searchImageBuildDir(path string) ImageBuildDirs {
	var ibds ImageBuildDirs

	// ディレクトリ名が'archive'の場合は探索をスキップする
	skipDirFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			err = fmt.Errorf("searchImageBuildDir.skipDirFunc Error:%v", err)
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
			ibds.ibds = append(ibds.ibds, ibd)
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
	if !isFile(filepath.Join(image_path, "Makefile")) {
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
	inames := make([]string, 0, len(ibd.dirTags)+1)
	for _, tag := range ibd.dirTags {
		inames = append(inames, fmt.Sprintf("%s:%s", ibd.dirImage, tag))
	}
	inames = append(inames, fmt.Sprintf("%s:%s", ibd.dirImage, "latest"))
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

// Docker imageをビルドするためのmake targetを指定する文字列を出力する
func (ibd *ImageBuildDir) BuildMakeInstruction(tag string) string {
	img_dir := filepath.Join(ibd.dirParent, ibd.dirImage)
	if tag == "latest" {
		return fmt.Sprintf("make -C %s latest\n", img_dir)
	}
	return fmt.Sprintf("make -C %s cache/%s.log\n", img_dir, tag)
}

func (ibd *ImageBuildDir) BuildDockerTaggingInstruction(tag string, new_tag string) string {
	if new_tag == "" || new_tag == "latest" || tag == new_tag {
		return ""
	}
	return fmt.Sprintf("docker tag %s:%s %s:%s\n", ibd.dirImage, tag, ibd.dirImage, new_tag)
}

type ImageBuildDirs struct {
	ibds []ImageBuildDir
	m    map[string]int
}

func (ibds *ImageBuildDirs) makeMap() {
	ibds.m = make(map[string]int)
	for i, ibd := range ibds.ibds {
		for _, iname := range ibd.ImageNames() {
			ibds.m[iname] = i
		}
	}
}

func (ibds ImageBuildDirs) ImageNames() []string {
	var inames []string
	for _, ibd := range ibds.ibds {
		inames = append(inames, ibd.ImageNames()...)
	}
	return inames
}

// ImageBuildDirのスライスについて、Dependencyのスライスを返す
func (ibds ImageBuildDirs) findDependencyFromDockerfiles() []Dependency {
	var deps []Dependency
	for _, ibd := range ibds.ibds {
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
