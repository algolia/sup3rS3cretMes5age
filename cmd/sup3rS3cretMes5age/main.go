// Package main provides the entry point for the sup3rS3cretMes5age application,
// a secure self-destructing message service using HashiCorp Vault as a backend.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/algolia/sup3rS3cretMes5age/internal"
)

// version holds the application version string, injected at build time via ldflags.
var version = ""

func main() {
	versionFlag := flag.Bool("version", false, "Print version")
	flag.Parse()
	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	conf := internal.LoadConfig()
	internal.Serve(conf)
}
