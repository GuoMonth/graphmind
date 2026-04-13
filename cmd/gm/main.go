package main

import (
	"github.com/senguoyun-guosheng/graphmind/internal/cli"
)

// version is set via -ldflags at build time.
var version = "dev"

func main() {
	cli.Execute(version)
}
