package promptsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncRendersMarkdownAndTargetTemplate(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "promptsync.yaml", `version: 1
source: prompts/master.md
variables:
  Project: Example App
targets:
  - path: .github/copilot-instructions.md
    template: templates/copilot.tmpl
`)
	writeTestFile(t, root, "prompts/master.md", "# {{ .Project }}\n\nShared rules.\n")
	writeTestFile(t, root, "templates/copilot.tmpl", "<!-- generated -->\n{{ .Content }}")

	written, err := Sync(filepath.Join(root, "promptsync.yaml"))
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if len(written) != 1 || written[0] != ".github/copilot-instructions.md" {
		t.Fatalf("Sync() paths = %v", written)
	}

	output, err := os.ReadFile(filepath.Join(root, ".github", "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("read generated target: %v", err)
	}
	if got := string(output); !strings.Contains(got, "<!-- generated -->") || !strings.Contains(got, "# Example App") {
		t.Fatalf("generated output = %q", got)
	}
}

func TestInspectReportsMissingCurrentAndModifiedTargets(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "promptsync.yaml")
	writeTestFile(t, root, "promptsync.yaml", `version: 1
source: prompts/master.md
targets:
  - path: CLAUDE.md
`)
	writeTestFile(t, root, "prompts/master.md", "Shared rules.\n")

	statuses, err := Inspect(configPath)
	if err != nil {
		t.Fatalf("Inspect() for missing target error = %v", err)
	}
	if len(statuses) != 1 || statuses[0].State != TargetMissing {
		t.Fatalf("Inspect() for missing target = %v", statuses)
	}
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("Inspect() created missing target, stat error = %v", err)
	}

	if _, err := Sync(configPath); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	statuses, err = Inspect(configPath)
	if err != nil {
		t.Fatalf("Inspect() for current target error = %v", err)
	}
	if statuses[0].State != TargetCurrent {
		t.Fatalf("Inspect() state = %q, want %q", statuses[0].State, TargetCurrent)
	}

	writeTestFile(t, root, "CLAUDE.md", "manual edit\n")
	statuses, err = Inspect(configPath)
	if err != nil {
		t.Fatalf("Inspect() for modified target error = %v", err)
	}
	if statuses[0].State != TargetModified {
		t.Fatalf("Inspect() state = %q, want %q", statuses[0].State, TargetModified)
	}
}

func TestConfigRejectsDuplicateTargets(t *testing.T) {
	config := Config{
		Version: 1,
		Source:  "prompts/master.md",
		Targets: []Target{{Path: "CLAUDE.md"}, {Path: "./CLAUDE.md"}},
	}
	if err := config.validate(); err == nil {
		t.Fatal("validate() accepted duplicate target paths")
	}
}

func TestInitDoesNotOverwriteExistingFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "promptsync.yaml", "user config")
	if err := Init(root); err == nil {
		t.Fatal("Init() overwrote an existing file")
	}
}

func writeTestFile(t *testing.T, root, path, contents string) {
	t.Helper()
	fullPath := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create test directory: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}
