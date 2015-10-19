package main

import (
	"flag"
	"testing"

	"github.com/barnybug/cli53"
)

// Test started when the test binary is started. Only calls main.
func TestSystem(t *testing.T) {
	args := append([]string{"cli53"}, flag.Args()...)
	exitCode := cli53.Main(args)
	if exitCode != 0 {
		t.Errorf("exit code: %d\n", exitCode)
	}
}
