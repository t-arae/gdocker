package main

import (
	"bufio"
	"log/slog"
	"os"
	"strings"
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
