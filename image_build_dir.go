package main

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Represent a docker image.
// Name and Tag must not contain semicolon ";" due to the design of this tool.
type DockerImage struct {
	Name     string // "ubuntu_a" (image name)
	Tag      string // "latest" (tag name)
	IsRoot   bool   // true when the image's Dockerfile has no parent image
	IsLatest bool
}

var (
	ErrParseImageName     = errors.New("image name parse error.")
	ErrCheckBuildImageDir = errors.New("build image dir check error.")
)

// DockerImage constructer
// Parse an image name (e.g. ubuntu_a:22.04, ubuntu_a) and return a DockerImage.
// If the tag was not specified, the tag will be set as "latest".
// This function checks belows.
// - the image name does not contain any semicolon.
// - the image name only has one or no colon.
// - the image name and tag are not blanks.
func NewDockerImage(image_name string) (DockerImage, error) {
	var d DockerImage

	// Check semicolon
	if strings.Index(image_name, ";") != -1 {
		return d, fmt.Errorf("%w cannot have semicolon. '%s'", ErrParseImageName, image_name)
	}

	// Check colon. if no colon included, then set tag as 'latest'.
	separated := strings.Split(image_name, ":")
	switch len(separated) {
	case 2:
		// correct. passed.
	case 1:
		separated = append(separated, "latest")
	default:
		return d, fmt.Errorf("%w must have one or zero colon. '%s'", ErrParseImageName, image_name)
	}

	// Check blank name or tag.
	if len(separated[0]) == 0 || len(separated[1]) == 0 {
		return d, fmt.Errorf("%w invalid name or tag. '%s'", ErrParseImageName, image_name)
	}

	d.Name = separated[0]
	d.Tag = separated[1]
	return d, nil
}

// Return the name:tag of Dockerimage
func (d DockerImage) String() string {
	return d.Name + ":" + d.Tag
}

func Strings(ds []DockerImage) []string {
	names := make([]string, len(ds))
	for _, d := range ds {
		names = append(names, d.String())
	}
	return names
}

// Represent a docker image building directory.
// Name and Tag must not contain semicolon ";" due to this tool design.
type ImageBuildDir struct {
	dirParent string
	dirImage  string
	dirTags   []string
	tagLatest int
	//imgs      []DockerImage
	Deps []Dependency
}

// Check whether the directory specified by the arguments is a valid image build directory, and return ImageBuildDir
// This function check bellows.
// - {building directory}/Makefile
// - {building directory}/{tag}/Dockerfile
// - # of tag ≧ 1
// - image dependencies (warning only)
// - latest image tag
func NewImageBuildDir(parent string, image string) (ImageBuildDir, error) {
	var ibd ImageBuildDir
	ibd.dirParent = parent
	ibd.dirImage = image
	dir := ibd.Directory()

	// if the dir has no Makefile, then return false
	if !isFile(filepath.Join(dir, "Makefile")) {
		return ibd, fmt.Errorf("%w could not find Makefile from '%s'", ErrCheckBuildImageDir, dir)
	}

	// Search tags and their dependencies
	skip_func := func(path string, d fs.DirEntry, err error) error {
		// search tags from direct child directories which has a Dockerfile.
		if filepath.Dir(path) == dir && isFile(filepath.Join(path, "Dockerfile")) {
			ibd.dirTags = append(ibd.dirTags, filepath.Base(path))
			// Search dependencies from the Dockerfile.
			deps, err := findDependenciesFromDockerfile(filepath.Join(path, "Dockerfile"))
			if err != nil {
				slog.Warn(err.Error())
			} else {
				ibd.Deps = append(ibd.Deps, deps...)
			}
			return filepath.SkipDir
		}
		return nil
	}
	if err := filepath.WalkDir(dir, skip_func); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	// if no tags found, then return false
	if len(ibd.dirTags) == 0 {
		return ibd, fmt.Errorf("%w could not find tags from '%s'.", ErrCheckBuildImageDir, dir)
	}

	// Makefileを読み込んで、latest tagが依存しているtagを探す
	lines := findLines(filepath.Join(dir, "Makefile"), "LATEST_VERSION = ")
	if len(lines) != 1 {
		return ibd, fmt.Errorf("%w invalid latest version for '%s': %v", ErrCheckBuildImageDir, dir, lines)
	}
	latest_ver := strings.TrimPrefix(lines[0], "LATEST_VERSION = ")
	idx_latest := slices.Index(ibd.dirTags, latest_ver)
	if idx_latest == -1 {
		return ibd, fmt.Errorf("%w the latest tag '%s' of '%s' didn't match any in %v", ErrCheckBuildImageDir, latest_ver, dir, ibd.dirTags)
	}

	ibd.tagLatest = idx_latest
	return ibd, nil
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
// FROM [--platform=<プラットフォーム>] <イメージ名> [AS <名前>]
// FROM [--platform=<プラットフォーム>] <イメージ名>[:<タグ>] [AS <名前>]
// FROM [--platform=<プラットフォーム>] <イメージ名>[@<ダイジェスト>] [AS <名前>]
func findDependenciesFromDockerfile(dfile string) ([]Dependency, error) {
	dir_tag := filepath.Dir(dfile)
	dir_image := filepath.Dir(dir_tag)

	var deps []Dependency
	left, err := NewDockerImage(filepath.Base(dir_image) + ":" + filepath.Base(dir_tag))
	if err != nil {
		return deps, err
	}

	lines := findLines(dfile, "FROM ")
	for _, v := range lines {
		li := strings.Split(strings.TrimPrefix(v, "FROM "), " ")
		var imgname string
		if strings.HasPrefix(li[0], "--platform=") {
			imgname = li[1]
		} else {
			imgname = li[0]
		}
		right, err := NewDockerImage(imgname)
		if err != nil {
			return deps, err
		}
		deps = append(deps, Dependency{left, right})
	}
	return deps, nil
}

func (ibd *ImageBuildDir) Directory() string {
	return filepath.Join(ibd.dirParent, ibd.dirImage)
}

// Returns a slice of string to remove docker images in the directory
func (ibd *ImageBuildDir) BuildCleanInstruction(tag string) []string {
	return []string{"-C", ibd.Directory(), fmt.Sprintf("clean-%s", tag)}
}

// Returns a slice of string to build the docker image specified by the tag
func (ibd *ImageBuildDir) BuildMakeInstruction(tag string) []string {
	if tag == "latest" {
		return []string{"-C", ibd.Directory(), "latest"}
	}
	return []string{"-C", ibd.Directory(), fmt.Sprintf("cache/%s.log", tag)}
}

// Returns a slice of string to tag the docker image with new tag
func (ibd *ImageBuildDir) BuildTaggingInstruction(tag string, new_tag string) ([]string, bool) {
	if new_tag == "" || new_tag == "latest" || tag == new_tag {
		return []string{}, false
	}
	return []string{"tag", fmt.Sprintf("%s:%s", ibd.dirImage, tag), fmt.Sprintf("%s:%s", ibd.dirImage, new_tag)}, true
}

// This contains a slice of ImageBuildDir and a map to search index from image name
type ImageBuildDirs struct {
	ibds       []ImageBuildDir
	mapNameTag map[string]int
	mapName    map[string]int
}

// Search ImageBuildDir from the specified directory and return ImageBuildDirs
func searchImageBuildDir(path string, skip string) ImageBuildDirs {
	var ibds ImageBuildDirs

	skip_func := func(path string, d fs.DirEntry, err error) error {
		// if `skip` is not "", then skips search when the basename is matched to `skip`.
		if skip != "" && filepath.Base(path) == skip {
			return filepath.SkipDir
		}

		if ibd, err := NewImageBuildDir(filepath.Dir(path), filepath.Base(path)); err == nil {
			ibds.ibds = append(ibds.ibds, ibd)
			return filepath.SkipDir
		} else {
			slog.Debug(err.Error())
		}
		return nil
	}
	if err := filepath.WalkDir(path, skip_func); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	return ibds
}

func (ibds *ImageBuildDirs) makeMap() {
	ibds.mapNameTag = make(map[string]int)
	for i, ibd := range ibds.ibds {
		for _, iname := range ibd.ImageNames() {
			ibds.mapNameTag[iname] = i
		}
	}
	ibds.mapName = make(map[string]int)
	for i, ibd := range ibds.ibds {
		if _, exist := ibds.mapName[ibd.dirImage]; exist {
			slog.Error(fmt.Sprintf("image name is duplicated: '%s'", ibd.dirImage))
			os.Exit(1)
		}
		ibds.mapName[ibd.dirImage] = i
	}
}

func (ibds *ImageBuildDirs) ImageNames() []string {
	var inames []string
	for _, ibd := range ibds.ibds {
		inames = append(inames, ibd.ImageNames()...)
	}
	return inames
}

// Returns a slice of Dependency of ImageBuildDirs
// An ibds instance from searchImageBuildDir() should satisfy requirements.
func (ibds *ImageBuildDirs) Dependencies() []Dependency {
	var deps []Dependency
	for _, ibd := range ibds.ibds {
		deps = append(deps, ibd.Deps...)

		// append the latest tag dependency
		deps = append(deps, Dependency{
			DockerImage{Name: ibd.dirImage, Tag: "latest"},
			DockerImage{Name: ibd.dirImage, Tag: ibd.dirTags[ibd.tagLatest]},
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
