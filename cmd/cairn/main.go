// Command cairn is an issue tracker for coding agents first and people second.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/seriouslysean/cairn/internal/cli"
)

var version = "0.0.0-dev"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "cairn:", err)
		return 1
	}
	return cli.Run(args, cli.Env{Version: version, Dir: dir, Stdout: stdout, Stderr: stderr, Getenv: os.Getenv})
}
