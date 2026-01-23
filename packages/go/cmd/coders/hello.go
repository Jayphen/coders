package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newHelloCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hello",
		Short: "Print hello world",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("hello world")
		},
	}
}
