package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("coders %s\n", Version)
			fmt.Printf("  go: %s\n", runtime.Version())
			fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}
}
