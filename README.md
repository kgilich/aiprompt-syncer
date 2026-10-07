# PromptSync

PromptSync keeps project instructions in one Markdown source and renders them into the files expected by tools such as GitHub Copilot, Claude Code, and Cursor.

## Requirements

- Go 1.22 or newer

## Quick start

Install the CLI from this checkout:

```powershell
go install ./cmd/promptsync
```

Then, in the project where you want the generated instruction files, run `promptsync init` followed by `promptsync sync`. For local development, run `go run ./cmd/promptsync init` and `go run ./cmd/promptsync sync` from this repository.

`init` creates `promptsync.yaml`, `prompts/master.md`, and starter templates. It refuses to overwrite any existing scaffold file. Edit the Markdown source and YAML variables, then run `sync` to write all configured targets.

## Configuration

```yaml
version: 1
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

Paths are resolved relative to the configuration file. The source and target templates use Go's `text/template` syntax. Source templates can reference YAML values such as `{{ .Project }}`. Target templates receive the rendered source as `{{ .Content }}` and can also access configured variables.

Commands:

- `promptsync init [-dir PATH]` creates a starter prompt library.
- `promptsync sync [-config PATH]` renders configured targets.
- `promptsync help` prints command usage.

Generated targets are overwritten by `sync`; keep edits in the source or templates.

## Development

```powershell
go test ./...
go build ./cmd/promptsync
```

API-based push/pull is TODO. For now, use `git push` to update the repository and `git pull` to get changes from others.
