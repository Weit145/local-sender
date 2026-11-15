package main

import (
	"fmt"
	"github.com/Weit145/local-sender/internal/cli"
	"os"
)

func main() {
	if err := cli.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
