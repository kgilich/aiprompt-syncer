package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncCheckReportsDriftWithoutWriting(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "promptsync.yaml")
	if err := os.WriteFile(configPath, []byte("version: 1\nsource: master.md\ntargets:\n  - path: CLAUDE.md\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "master.md"), []byte("rules\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	err := run([]string{"sync", "--check", "-config", configPath})
	if err == nil {
		t.Fatal("run(sync --check) succeeded with a missing target")
	}
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("sync --check wrote target, stat error = %v", err)
	}
}

func TestSyncRequiresForceForModifiedTarget(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "promptsync.yaml")
	if err := os.WriteFile(configPath, []byte("version: 1\nsource: master.md\ntargets:\n  - path: CLAUDE.md\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "master.md"), []byte("generated\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	targetPath := filepath.Join(root, "CLAUDE.md")
	if err := os.WriteFile(targetPath, []byte("manual edit\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	if err := run([]string{"sync", "-config", configPath}); err == nil {
		t.Fatal("sync unexpectedly overwrote a modified target")
	}
	contents, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read protected target: %v", err)
	}
	if string(contents) != "manual edit\n" {
		t.Fatalf("sync changed protected target to %q", contents)
	}

	if err := run([]string{"sync", "--force", "-config", configPath}); err != nil {
		t.Fatalf("sync --force error: %v", err)
	}
	contents, err = os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read forced target: %v", err)
	}
	if string(contents) != "generated\n" {
		t.Fatalf("sync --force target = %q", contents)
	}
}
