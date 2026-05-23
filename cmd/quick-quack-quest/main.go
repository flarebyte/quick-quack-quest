// purpose: Provide the executable entrypoint that runs the CLI and returns process-level exit codes.
// responsibilities: Construct the root command, execute it, and print fatal CLI errors to stderr.
// architecture notes: Business logic is intentionally delegated to internal packages so main stays thin and stable.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/flarebyte/quick-quack-quest/internal/cli"
)

func run(args []string, stderr io.Writer) int {
	root := cli.NewRootCommand()
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

var osExit = os.Exit

func main() {
	osExit(run(os.Args[1:], os.Stderr))
}
