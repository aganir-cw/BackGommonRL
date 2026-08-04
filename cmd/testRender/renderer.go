package main

import (
	"fmt"
	"golearner/engine"
)

func main() {
	board := engine.Start()
	fmt.Println(board.Render())
}
