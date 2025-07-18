package main

import (
	"fmt"
	"os"

	"github.com/jstein/qmp/cmd"
)

func main() {
	// Execute the root command
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
