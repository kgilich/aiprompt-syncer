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
