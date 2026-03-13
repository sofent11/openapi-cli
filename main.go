package main

import (
	"fmt"
	"os"

	"github.com/sofent/openapi-cli/internal/cli"
)

func main() {
	root := cli.NewRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, cli.FormatCommandError(root, err))
		os.Exit(1)
	}
}
