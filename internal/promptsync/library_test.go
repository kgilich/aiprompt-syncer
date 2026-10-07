package promptsync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateGitRepositoryClonesAndFetchesRef(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}

	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	working := filepath.Join(root, "working")
	cache := filepath.Join(root, "cache", "library")
	runGitTest(t, gitPath, "init", "--bare", remote)
	runGitTest(t, gitPath, "init", working)
	runGitTest(t, gitPath, "-C", working, "-c", "user.name=PromptSync Test", "-c", "user.email=promptsync@example.invalid", "commit", "--allow-empty", "-m", "initial")
	runGitTest(t, gitPath, "-C", working, "branch", "-M", "main")
	runGitTest(t, gitPath, "-C", working, "remote", "add", "origin", remote)
	runGitTest(t, gitPath, "-C", working, "push", "-u", "origin", "main")

	if err := updateGitRepository(gitPath, remote, "main", cache); err != nil {
		t.Fatalf("initial clone: %v", err)
	}
	initial, err := runGit(gitPath, "-C", cache, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("read initial revision: %v", err)
	}

	if err := os.WriteFile(filepath.Join(working, "master.md"), []byte("updated prompt\n"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	runGitTest(t, gitPath, "-C", working, "add", "master.md")
	runGitTest(t, gitPath, "-C", working, "-c", "user.name=PromptSync Test", "-c", "user.email=promptsync@example.invalid", "commit", "-m", "update prompt")
	runGitTest(t, gitPath, "-C", working, "push", "origin", "main")

	if err := updateGitRepository(gitPath, remote, "main", cache); err != nil {
		t.Fatalf("fetch update: %v", err)
	}
	updated, err := runGit(gitPath, "-C", cache, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("read updated revision: %v", err)
	}
	if strings.TrimSpace(initial) == strings.TrimSpace(updated) {
		t.Fatal("update did not advance cached revision")
	}
	contents, err := os.ReadFile(filepath.Join(cache, "master.md"))
	if err != nil {
		t.Fatalf("read updated prompt: %v", err)
	}
	if string(contents) != "updated prompt\n" {
		t.Fatalf("cached prompt = %q", contents)
	}
}

func TestConfigRejectsNonHTTPSLibraryRepository(t *testing.T) {
	config := Config{
		Version: 1,
		Library: &Library{Repository: "git@github.com:owner/prompts.git", Ref: "main"},
		Source:  "prompts/master.md",
		Targets: []Target{{Path: "CLAUDE.md"}},
	}
	if err := config.validate(); err == nil {
		t.Fatal("validate() accepted a non-HTTPS repository")
	}
}

func runGitTest(t *testing.T, gitPath string, args ...string) {
	t.Helper()
	if _, err := runGit(gitPath, args...); err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
}
