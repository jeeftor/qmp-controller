package main

import (
	"fmt"
	"os"

	"github.com/jeeftor/qmp-controller/cmd"
)

func main() {
	// Execute the root command
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
