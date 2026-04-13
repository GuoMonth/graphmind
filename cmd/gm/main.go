package main

import (
	"os"

	"github.com/senguoyun-guosheng/graphmind/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
