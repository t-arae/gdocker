package main

import (
	"archive/tar"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	} else {
		return info.IsDir()
	}
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	} else {
		return !info.IsDir()
	}
}

func relativeTo(path string, base string) string {
	// Convert path to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	// Convert base to absolute path
	absBase, err := filepath.Abs(base)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	// Get the relative path
	relPath, err := filepath.Rel(absBase, absPath)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	return relPath
}

func copyFile(source string, dest string) {
	data, err := os.ReadFile(source)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	if isFile(dest) {
		slog.Warn(fmt.Sprintf("%s is already exist. skipped.", dest))
		return
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0777); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	f, err := os.Create(dest)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	defer f.Close()
	_, err = f.Write(data)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func createTarFile(source string, dest string) {
	f, err := os.Create(dest)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	if err := filepath.Walk(source, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if err := tw.WriteHeader(&tar.Header{
			Name:    relativeTo(path, filepath.Dir(source)),
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		}); err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(tw, f); err != nil {
			return err
		}

		return nil
	}); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func mkDirAll(dir string) {
	if err := os.MkdirAll(dir, 0777); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
