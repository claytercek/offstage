# offstage

Personal notes, colocated with your projects.

## Why

Every project ends up with files that are yours, not the team's. The note about why you went with that approach. The scratch file you keep open during a feature. Reminders about which environment variable does what.

They're useful. They don't belong in the repo.

offstage syncs them across machines using a private git repository. Your notes live next to the code and follow you to whatever machine you're on. It doesn't care what you track — point it at whatever you want.

## How it works

offstage clones a private git repository to your machine as the **sync store**. When you `push`, your tracked files are committed to a branch named `<project-id>/<git-branch>`. When you `pull`, they're written back to the local filesystem. Each project branch gets its own slice of the store; nothing flows between branches until you run `merge`.

## Quick start

```sh
# 1. Initialize once per machine
offstage init git@github.com:you/my-private-store.git

# 2. Configure a project
cd ~/repos/my-project
offstage track "notes/**"

# 3. Push from the project
offstage push

# 4. Pull on another machine
offstage pull
```

## Commands

| Command | Description |
|---|---|
| `offstage init <git-url>` | Clone the sync store and write `~/.config/offstage/config.toml` |
| `offstage push` | Commit tracked files to the sync store |
| `offstage pull` | Write files from the sync store to the local filesystem |
| `offstage track <pattern>` | Add a Git ignore pattern to `.offstagerc.toml` |
| `offstage untrack <pattern>` | Remove a Git ignore pattern from `.offstagerc.toml` |
| `offstage git-exclude` | Sync tracked patterns to `.git/info/exclude` |
| `offstage diff [<branch>]` | Show diff between local and store state; with a branch arg, diff two store branches |
| `offstage merge <branch>` | Merge a branch context into the current one (run after a PR lands) |
| `offstage status` | Show which tracked files differ from the store |
| `offstage git <args...>` | Run a git command inside the sync store |

### Flags

- `--dry-run` on `push`/`pull`: print what would change without writing anything
- `--sync-exclude` on `track`/`untrack`: also sync tracked patterns to `.git/info/exclude`

## Tracked files

Projects do not track anything by default. Configure tracked files with `.offstagerc.toml` in the repository root:

```toml
name = "my-project"          # optional; defaults to normalized git remote URL

include = [
  "notes/**",
  "docs/adr/**",
]

exclude = [
  "notes/scratch.md",
]

[git_exclude]
auto_sync = true             # optional; defaults to false
```

Patterns use Git ignore semantics. For example, `*.md` matches Markdown files at any depth, while `/README.md` only matches a root file.

Configure the sync store in `~/.config/offstage/config.toml`:

```toml
store_url  = "git@github.com:you/my-private-store.git"
store_path = "/home/you/.local/share/offstage/store"
```

## Branch workflow

offstage stores a branch context per project branch. After a PR lands, reconcile the merged branch back into your main branch:

```sh
git checkout main
offstage merge my-feature-branch
```

`pull` will warn you if there are unreconciled branches you haven't merged yet.

## Installation

```sh
go install github.com/claytercek/offstage/cmd/offstage@latest
```
