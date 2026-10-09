# PromptSync

**One source of truth for AI coding instructions.** Write your project guidance once in Markdown, then render it into the instruction files your tools already use.

[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![CI](https://github.com/kgilich/promptsync/actions/workflows/ci.yml/badge.svg)](https://github.com/kgilich/promptsync/actions/workflows/ci.yml)

Generate files for GitHub Copilot, Claude Code, Cursor, and other tools from one prompt library. Keep it local, or share it across projects through Git. A committed lockfile makes every project use the same library revision.

## Get started

### Local prompt library

Install PromptSync from this checkout:

```sh
go install ./cmd/promptsync
```

In the project where you want the generated instructions:

```sh
promptsync init
```

Edit `prompts/master.md`, then generate the configured instruction files:

```sh
promptsync sync
```

`init` creates a starter `promptsync.yaml`, Markdown source, and target templates. It will not overwrite existing scaffold files.

### Shared Git library

Add a remote library to `promptsync.yaml`:

```yaml
version: 1
library:
  repository: https://github.com/your-org/prompt-library.git
  ref: main
source: prompts/master.md
variables:
  Project: Example App
targets:
  - path: .github/copilot-instructions.md
    template: templates/copilot.tmpl
  - path: CLAUDE.md
    template: templates/claude.tmpl
  - path: .cursorrules
    template: templates/cursor.tmpl
```

Fetch the latest configured revision and render it:

```sh
promptsync update
promptsync sync
```

Commit `promptsync.lock` with your project. It records the exact library commit. On a fresh clone or CI runner, install that pinned revision without changing the lock:

```sh
promptsync install
promptsync sync --check
```

Run `promptsync update` only when you intentionally want to move to a newer revision, then review and commit the lockfile change.

## Commands

| Command | What it does |
| --- | --- |
| `promptsync init [-dir PATH]` | Create a starter prompt library. |
| `promptsync update [-config PATH]` | Fetch the configured Git library and update `promptsync.lock`. |
| `promptsync install [-config PATH]` | Install the exact commit from `promptsync.lock` without changing it. |
| `promptsync status [-config PATH]` | Report each target as `missing`, `current`, or `modified`. |
| `promptsync sync [-config PATH]` | Render and write all configured targets. |
| `promptsync sync --check [-config PATH]` | Check target freshness without writing; fail if any target is missing or modified. |
| `promptsync sync --force [-config PATH]` | Overwrite targets that differ from generated content. |
| `promptsync help` | Show command usage. |

## How it works

- **One source:** Markdown is the canonical prompt; Go templates adapt it for each target.
- **Consistent output:** Generated text always uses LF line endings, regardless of the source checkout's CRLF/LF settings.
- **Explicit network steps:** `update` advances the lock; `install` materializes the locked commit. `sync`, `status`, and `sync --check` do not access the network.
- **Reproducible projects:** `sync` and `status` use the commit in `promptsync.lock`.
- **Local or shared:** Without `library`, paths are relative to the config file. With `library`, source and template paths are relative to the cloned library.
- **Contained inputs:** Source and template paths, including symlinks, cannot escape the library or project root.
- **Safe defaults:** Modified files are not overwritten unless `sync --force` is given. Output paths, including symlink-resolved paths, must stay inside the project unless `allow_external_targets: true` is set in the config.
- **Git required for remote libraries:** Remote URLs must use HTTPS and identify a branch or tag. The repository is cached in the operating system's user cache directory.

`status` and `sync --check` are read-only. `sync` creates missing targets and leaves current ones alone. If a target was manually changed, inspect it with `status` and use `sync --force` only when you intend to discard those edits.

For CI using a remote library, restore the exact lock first, then verify that generated files are committed and current:

```sh
promptsync install
promptsync sync --check
```

## Develop

Supports Linux, macOS, and Windows. Requires Go 1.22 or newer; Git is needed only when using a remote library.

```sh
go test ./...
go build ./cmd/promptsync
```
