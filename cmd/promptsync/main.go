package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kgilich/aiprompt-syncer/internal/promptsync"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "promptsync:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}

	switch args[0] {
	case "init":
		flags := flag.NewFlagSet("init", flag.ContinueOnError)
		root := flags.String("dir", ".", "directory to initialize")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return usageError()
		}
		if err := promptsync.Init(*root); err != nil {
			return err
		}
		fmt.Printf("Initialized PromptSync in %s\n", *root)
		return nil
	case "sync":
		flags := flag.NewFlagSet("sync", flag.ContinueOnError)
		config := flags.String("config", "promptsync.yaml", "path to configuration file")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return usageError()
		}
		written, err := promptsync.Sync(*config)
		if err != nil {
			return err
		}
		for _, path := range written {
			fmt.Println("Wrote", path)
		}
		return nil
	case "help", "-h", "--help":
		fmt.Print("Usage: promptsync <init|sync> [options]\n\nCommands:\n  init   Create a starter library and configuration\n  sync   Render the library into configured target files\n")
		return nil
	default:
		return usageError()
	}
}

func usageError() error {
	return fmt.Errorf("usage: promptsync <init|sync> [options] (run 'promptsync help' for details)")
}
