package promptsync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Update(configPath string) (string, error) {
	config, _, err := loadConfig(configPath)
	if err != nil {
		return "", err
	}
	if config.Library == nil {
		return "", fmt.Errorf("no remote library configured")
	}

	cachePath, err := libraryCachePath(*config.Library)
	if err != nil {
		return "", err
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git is required to update a remote library: %w", err)
	}

	if err := updateGitRepository(gitPath, config.Library.Repository, config.Library.Ref, cachePath); err != nil {
		return "", err
	}

	revision, err := runGit(gitPath, "-C", cachePath, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(revision), nil
}

func libraryRoot(config Config, projectRoot string) (string, error) {
	if config.Library == nil {
		return projectRoot, nil
	}
	cachePath, err := libraryCachePath(*config.Library)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(cachePath, ".git")); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("remote library is not cached; run 'promptsync update' first")
		}
		return "", fmt.Errorf("inspect library cache: %w", err)
	}
	return cachePath, nil
}

func libraryCachePath(library Library) (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate user cache directory: %w", err)
	}
	key := sha256.Sum256([]byte(library.Repository + "\x00" + library.Ref))
	return filepath.Join(cacheRoot, "promptsync", "libraries", hex.EncodeToString(key[:])), nil
}

func runGit(gitPath string, args ...string) (string, error) {
	command := exec.Command(gitPath, args...)
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func updateGitRepository(gitPath, repository, ref, cachePath string) error {
	if _, err := os.Stat(filepath.Join(cachePath, ".git")); err == nil {
		if _, err := runGit(gitPath, "-C", cachePath, "fetch", "--depth=1", "origin", ref); err != nil {
			return err
		}
		if _, err := runGit(gitPath, "-C", cachePath, "checkout", "--detach", "FETCH_HEAD"); err != nil {
			return err
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect library cache: %w", err)
	}
	if _, err := os.Stat(cachePath); err == nil {
		return fmt.Errorf("library cache path exists but is not a Git repository: %s", cachePath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect library cache path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return fmt.Errorf("create library cache directory: %w", err)
	}
	if _, err := runGit(gitPath, "clone", "--depth=1", "--single-branch", "--branch", ref, "--", repository, cachePath); err != nil {
		return err
	}
	return nil
}
