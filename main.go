package main

import "github.com/charmbracelet/fang"

func main() {
	shark := theshark()
	_ = fang.Execute(shark.Context(), shark, fang.WithVersion("0.1.0"))
}
