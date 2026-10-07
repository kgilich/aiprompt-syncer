# PromptSync

**One source of truth for AI coding instructions.** Write your project guidance once in Markdown, then render it into the instruction files your tools already use.

[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![CI](https://github.com/kgilich/aiprompt-syncer/actions/workflows/ci.yml/badge.svg)](https://github.com/kgilich/aiprompt-syncer/actions/workflows/ci.yml)

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

Fetch the library and render it:

```sh
promptsync update
promptsync sync
```

Commit `promptsync.lock` with your project. It records the exact library commit, so teammates and CI can reproduce the same generated instructions. Run `promptsync update` when you intentionally want to move to a newer revision.

## Commands

| Command | What it does |
| --- | --- |
| `promptsync init [-dir PATH]` | Create a starter prompt library. |
| `promptsync update [-config PATH]` | Fetch the configured Git library and update `promptsync.lock`. |
| `promptsync status [-config PATH]` | Report each target as `missing`, `current`, or `modified`. |
| `promptsync sync [-config PATH]` | Render and write all configured targets. |
| `promptsync sync --check [-config PATH]` | Check target freshness without writing; fail if any target is missing or modified. |
| `promptsync help` | Show command usage. |

## How it works

- **One source:** Markdown is the canonical prompt; Go templates adapt it for each target.
- **Explicit updates:** `sync` never makes a network request. Fetch new library content with `update`.
- **Reproducible projects:** `sync` and `status` use the commit in `promptsync.lock`.
- **Local or shared:** Without `library`, paths are relative to the config file. With `library`, source and template paths are relative to the cloned library.
- **Git required for remote libraries:** Remote URLs must use HTTPS and identify a branch or tag. The repository is cached in the operating system's user cache directory.

`status` and `sync --check` are read-only. `sync` overwrites configured targets, including manual edits; inspect changes with `status` first.

## Develop

Requires Go 1.22 or newer. Git is needed only when using a remote library.

```sh
go test ./...
go build ./cmd/promptsync
```
