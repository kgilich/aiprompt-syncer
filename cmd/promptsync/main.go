package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kgilich/promptsync/internal/promptsync"
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
		check := flags.Bool("check", false, "check generated targets without writing files")
		force := flags.Bool("force", false, "overwrite modified targets")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return usageError()
		}
		if *check && *force {
			return fmt.Errorf("--check and --force cannot be used together")
		}
		if *check {
			statuses, err := promptsync.Inspect(*config)
			if err != nil {
				return err
			}
			printStatuses(statuses)
			for _, status := range statuses {
				if status.State != promptsync.TargetCurrent {
					return fmt.Errorf("generated targets are out of date; run 'promptsync sync'")
				}
			}
			return nil
		}
		written, err := promptsync.SyncWithOptions(*config, promptsync.SyncOptions{Force: *force})
		if err != nil {
			return err
		}
		for _, path := range written {
			fmt.Println("Wrote", path)
		}
		return nil
	case "update":
		flags := flag.NewFlagSet("update", flag.ContinueOnError)
		config := flags.String("config", "promptsync.yaml", "path to configuration file")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return usageError()
		}
		revision, err := promptsync.Update(*config)
		if err != nil {
			return err
		}
		fmt.Printf("Updated library to %s\n", revision)
		return nil
	case "install":
		flags := flag.NewFlagSet("install", flag.ContinueOnError)
		config := flags.String("config", "promptsync.yaml", "path to configuration file")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return usageError()
		}
		revision, err := promptsync.Install(*config)
		if err != nil {
			return err
		}
		fmt.Printf("Installed library at %s\n", revision)
		return nil
	case "status":
		flags := flag.NewFlagSet("status", flag.ContinueOnError)
		config := flags.String("config", "promptsync.yaml", "path to configuration file")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return usageError()
		}
		statuses, err := promptsync.Inspect(*config)
		if err != nil {
			return err
		}
		printStatuses(statuses)
		return nil
	case "help", "-h", "--help":
		fmt.Print("Usage: promptsync <init|update|install|status|sync> [options]\n\nCommands:\n  init               Create a starter library and configuration\n  update             Fetch the configured remote prompt library and advance the lock\n  install            Fetch the exact library revision from the lockfile\n  status             Show whether configured targets are current\n  sync               Render the library into configured target files\n  sync --check       Check target freshness without writing files\n  sync --force       Overwrite modified targets\n")
		return nil
	default:
		return usageError()
	}
}

func printStatuses(statuses []promptsync.TargetStatus) {
	for _, status := range statuses {
		fmt.Printf("%-8s %s\n", status.State, status.Path)
	}
}

func usageError() error {
	return fmt.Errorf("usage: promptsync <init|update|install|status|sync> [options] (run 'promptsync help' for details)")
}
