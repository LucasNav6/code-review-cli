// Package main is the entry point of the code-review CLI.
//
// Per the official Cobra user guide, main.go must stay bare: its only
// responsibility is to delegate execution to the cmd package. All command
// wiring (root, subcommands, flags) lives under cmd/.
package main

import "github.com/LucasNav6/code-review-cli/cmd"

func main() {
	cmd.Execute()
}
