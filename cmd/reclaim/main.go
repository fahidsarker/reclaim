package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/fahid/reclaim/internal/cli"
	"github.com/fahid/reclaim/internal/detect"
	_ "github.com/fahid/reclaim/internal/detect/builtin"
)

func main() {
	detect.MustLoadEmbedded()
	if err := cli.Execute(); err != nil {
		var ee *cli.ExitError
		if errors.As(err, &ee) {
			if ee.Message != "" {
				fmt.Fprintln(os.Stderr, ee.Message)
			}
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(cli.ExitCode(err))
	}
}
