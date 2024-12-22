package main

import (
	"log/slog"
	"os"
)

// DockerImageの依存関係を示す。FromがToに依存している。
type Dependency struct {
	From DockerImage
	To   DockerImage
}

func checkDependency(images []DockerImage, deps []Dependency) ([]DockerImage, map[string]struct{}) {
	// Initialize the graph.
	graph := NewGraph[string]()

	// Add edges.
	for _, dep := range deps {
		graph.AddEdge(dep.From.String(), dep.To.String())
	}

	roots := map[string]struct{}{}
	sorted := make([]string, 0, len(images))
	appeared := make(map[string]struct{}, len(images))
	for i, image := range images {
		exist := false
		if i != 0 {
			for _, called := range sorted {
				if image.String() == called {
					exist = true
					break
				}
			}
		}
		if image.Name == "" {
			continue
		}

		if !exist {
			imgnames_to_add, err := graph.TopSort(image.String())
			if err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
			if _, ok := roots[imgnames_to_add[0]]; !ok {
				roots[imgnames_to_add[0]] = struct{}{}
			}
			for _, imgname_to_add := range imgnames_to_add {
				if _, ok := appeared[imgname_to_add]; ok {
				} else {
					sorted = append(sorted, imgname_to_add)
					appeared[imgname_to_add] = struct{}{}
				}
			}
		}
	}

	img_sorted := make([]DockerImage, 0, len(sorted))
	for _, imgname := range sorted {
		d := NewDockerImage(imgname)
		if _, ok := roots[d.String()]; ok {
			d.IsRoot = true
		}
		img_sorted = append(img_sorted, d)
	}
	return img_sorted, roots
}
