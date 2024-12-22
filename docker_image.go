package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DockerImage struct {
	Name     string // "ubuntu_a" (image name)
	Tag      string // "latest" (tag name)
	Arch     string // "arm" or "x86_64" or "" (CPU architecture)
	Dir      string // "docker_image/arm/ubuntu_a" (directory)
	ExistDir bool   // true or false
	DirRoot  string // "docker_image"
	IsRoot   bool
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

	// NameからArchを判別して設定する
	if len(d.Name) < 2 {
		d.Arch = ""
	} else {
		suffix := d.Name[len(d.Name)-2:]
		switch suffix {
		case "_a":
			d.Arch = "arm"
		case "_x":
			d.Arch = "x86_64"
		default:
			d.Arch = ""
		}
	}

	// ビルドに必要なディレクトリが存在するかチェック
	d.CheckDirectory()

	return d
}

// DockerImageで設定されたディレクトリがあるかチェックして、ExistDirを設定する
func (d *DockerImage) CheckDirectory() {
	if d.Arch == "" {
		// Archが設定されていない場合はビルド対象ではない
		d.Dir, d.ExistDir = "", false
	} else {
		if d.Tag == "latest" {
			d.Dir = filepath.Join(d.Arch, d.Name)
		} else {
			d.Dir = filepath.Join(d.Arch, d.Name, d.Tag)
		}
		dir_full := filepath.Join(d.DirRoot, d.Dir)
		if f, err := os.Stat(dir_full); os.IsNotExist(err) || !f.IsDir() {
			d.ExistDir = false
		} else {
			d.ExistDir = true
		}
	}
}

// Dockerimageの名前（例: ubuntu_a:latest）を返す
func (d DockerImage) String() string {
	if d.Name == "" {
		return ""
	}
	if d.IsRoot {
		return d.Name + ":" + d.Tag + "[root]"
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

func (d DockerImage) BuildDockerTaggingInstruction(common_tag string) string {
	if common_tag == "" || common_tag == "latest" {
		return ""
	}
	if d.Tag == common_tag {
		return ""
	} else {
		return fmt.Sprintf("docker tag %s %s:%s\n", d.String(), d.Name, common_tag)
	}
}

// Docker imageをビルドするためのmake targetを指定する文字列を出力する
func (d DockerImage) BuildMakeInstruction() string {
	parent_dir := filepath.Dir(d.Dir)
	if d.Tag == "latest" {
		return fmt.Sprintf("make -C %s latest\n", d.Dir)
	}
	return fmt.Sprintf("make -C %s cache/%s.log\n", parent_dir, d.Tag)
}
