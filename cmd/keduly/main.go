// Command keduly is both the server (keduly serve) and the CLI client.
package main

import (
	"os"

	"github.com/Lewin671/keduly/internal/cli"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "serve" {
		os.Exit(serve(args[1:]))
	}
	os.Exit(cli.Run(args, cli.Env{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Version: version, Getenv: os.Getenv}))
}
