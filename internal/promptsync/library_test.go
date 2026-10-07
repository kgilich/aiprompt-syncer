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
	if err := os.WriteFile(filepath.Join(working, "master.md"), []byte("initial prompt\n"), 0o644); err != nil {
		t.Fatalf("write initial source file: %v", err)
	}
	runGitTest(t, gitPath, "-C", working, "add", "master.md")
	runGitTest(t, gitPath, "-C", working, "-c", "user.name=PromptSync Test", "-c", "user.email=promptsync@example.invalid", "commit", "-m", "initial")
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
	initial = strings.TrimSpace(initial)
	lockPath := filepath.Join(root, "promptsync.lock")
	if err := writeLibraryLock(lockPath, LibraryLock{Version: 1, Repository: "https://example.invalid/prompts.git", Ref: "main", Commit: initial}); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}
	lock, err := readLibraryLock(lockPath)
	if err != nil {
		t.Fatalf("read lockfile: %v", err)
	}
	if lock.Commit != initial {
		t.Fatalf("locked commit = %q, want %q", lock.Commit, initial)
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

	if err := checkoutLibraryCommit(gitPath, cache, lock.Commit); err != nil {
		t.Fatalf("checkout locked revision: %v", err)
	}
	lockedRevision, err := runGit(gitPath, "-C", cache, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("read locked revision: %v", err)
	}
	if strings.TrimSpace(lockedRevision) != lock.Commit {
		t.Fatalf("checked out revision = %q, want locked %q", strings.TrimSpace(lockedRevision), lock.Commit)
	}
	contents, err = os.ReadFile(filepath.Join(cache, "master.md"))
	if err != nil {
		t.Fatalf("read locked prompt: %v", err)
	}
	if string(contents) != "initial prompt\n" {
		t.Fatalf("locked prompt = %q, want initial prompt", contents)
	}
}

func TestLibraryRootRequiresLockfile(t *testing.T) {
	config := Config{Library: &Library{Repository: "https://example.com/prompts.git", Ref: "main"}}
	if _, err := libraryRoot(config, t.TempDir()); err == nil || !strings.Contains(err.Error(), "promptsync update") {
		t.Fatalf("libraryRoot() error = %v, want update instruction", err)
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
