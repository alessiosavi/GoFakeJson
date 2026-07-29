// Command gofakejson generates JSON test data from a template.
package main

import (
	"os"

	"github.com/alessiosavi/GoFakeJson/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}
