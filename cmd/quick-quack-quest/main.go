// purpose: Provide the executable entrypoint that runs the CLI and returns process-level exit codes.
// responsibilities: Construct the root command, execute it, and print fatal CLI errors to stderr.
// architecture notes: Business logic is intentionally delegated to internal packages so main stays thin and stable.
package main

import (
	"fmt"
	"os"

	"github.com/flarebyte/quick-quack-quest/internal/cli"
)

func main() {
	root := cli.NewRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
