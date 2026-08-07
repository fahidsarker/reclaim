package main

import (
	"fmt"
	"os"

	"github.com/fahid/reclaim/internal/cli"
	"github.com/fahid/reclaim/internal/detect"
)

func main() {
	detect.MustLoadEmbedded()
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
