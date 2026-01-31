package main

import (
	"errors"
	"fmt"
	"os"
)

func main() {
	if err := Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "skv:", err)
		if errors.Is(err, errUsage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
