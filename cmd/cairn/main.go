// Command cairn is an issue tracker for coding agents first and people second.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

var version = "0.0.0-dev"

const usage = `cairn: an issue tracker for agents first.

Usage: cairn <command> [flags]

Commands:
  version   print the version (--json for JSON)
  help      print this help

More commands arrive with each build step; see docs/spec.md.
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	switch args[0] {
	case "version", "--version":
		if hasFlag(args[1:], "--json") {
			_ = json.NewEncoder(stdout).Encode(map[string]any{"ok": true, "data": map[string]string{"version": version}})
		} else {
			fmt.Fprintln(stdout, "cairn", version)
		}
		return 0
	}
	fmt.Fprintf(stderr, "cairn: unknown command %q (run `cairn help`)\n", args[0])
	return 2
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}
