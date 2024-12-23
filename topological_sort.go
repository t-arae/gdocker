package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Node[T comparable] map[T]struct{}

type Graph[T comparable] struct {
	// 隣接しているNodeを表す。Node名のマップで、要素がNodeのマップ
	// adjacencyの各要素がグラフ内のNodeを表し、各要素の長さが入次数をしめす。
	adjacency map[T]Node[T]
}

func NewGraph[T comparable]() *Graph[T] {
	return &Graph[T]{
		adjacency: make(map[T]Node[T]),
	}
}

func (g *Graph[T]) HasNode(node T) bool {
	_, exists := g.adjacency[node]
	return exists
}

func (g *Graph[T]) AddNode(node T) {
	if !g.HasNode(node) {
		g.adjacency[node] = make(Node[T])
	}
}

// エッジを追加する (from -> to)
func (g *Graph[T]) AddEdge(from T, to T) {
	g.AddNode(from)
	g.AddNode(to)
	g.adjacency[from][to] = struct{}{}
}

// トポロジカルソートを行う
// 引数のノードを起点として全ての到達可能ノードをDFS
// サイクルがあればエラーを返す
func (g *Graph[T]) TopSort(start T) ([]T, error) {
	var result []T

	if !g.HasNode(start) {
		return result, nil
	}

	visited := make(map[T]bool)
	active := make(map[T]bool) // 現在の探索経路上にあるか
	var dfs func(T) error
	dfs = func(curr T) error {
		// すでに処理済みならスキップ
		if visited[curr] {
			return nil
		}
		if active[curr] {
			// サイクル検出。activeの中からcurrが出現するまでのノードを抽出
			cycle := g.extractCyclePath(active, curr)
			strCycle := make([]string, len(cycle))
			for i, c := range cycle {
				strCycle[i] = fmt.Sprintf("%v", c)
			}
			slog.Error(fmt.Sprintf("Cycle error: %s\n", strings.Join(strCycle, " -> ")))
			os.Exit(1)
		}

		active[curr] = true
		for nxt := range g.adjacency[curr] {
			if err := dfs(nxt); err != nil {
				return err
			}
		}
		active[curr] = false
		visited[curr] = true
		result = append(result, curr)
		return nil
	}

	if err := dfs(start); err != nil {
		return nil, err
	}

	return result, nil
}

func (g *Graph[T]) extractCyclePath(active map[T]bool, start T) []T {
	var cycle []T
	for k := range active {
		cycle = append(cycle, k)
	}
	cycle = append(cycle, start)
	return cycle
}

//func main() {
//	g := NewGraph[string]()
//	g.AddNode("A")
//	g.AddNode("B")
//	g.AddEdge("A", "B")
//	g.AddEdge("B", "C")
//	g.AddEdge("C", "D")
//
//	order, err := g.TopSort("A")
//	if err != nil {
//		fmt.Println(err)
//	} else {
//		fmt.Println("Topological Order:", order)
//	}
//}
