package main

import (
	"os"

	"github.com/barnybug/cli53"
)

func main() {
	exitCode := cli53.Main(os.Args)
	os.Exit(exitCode)
}
