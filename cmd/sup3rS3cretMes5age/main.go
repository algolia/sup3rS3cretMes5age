package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/algolia/sup3rS3cretMes5age/internal"
)

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
