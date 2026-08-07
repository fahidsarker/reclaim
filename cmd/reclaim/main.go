package main

import (
	"fmt"
	"os"

	"github.com/fahid/reclaim/internal/cli"
	_ "github.com/fahid/reclaim/internal/detect/builtin"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
