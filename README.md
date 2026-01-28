# Arbor

Arbor is a self-contained binary for managing git worktrees to assist with agentic development of applications. It is cross-project, cross-language, and cross-environment compatible.

## Development

All development occurs inside a worktree:

```bash
# Create a worktree for development
arbor work feature/new-feature
cd feature-new-feature

# Make changes, test, commit
go test ./...
arbor work another-feature  # Create another if needed

# When done with a worktree
cd ..
arbor remove feature-new-feature
```

## Installation

```bash
# Clone and build
git clone git@github.com:michaeldyrynda/arbor.git
cd arbor
go build -o arbor ./cmd/arbor

# Or install via Homebrew (coming soon)
brew install arbor
```

### Shell Integration (Optional)

To automatically `cd` into new worktrees after running `arbor work`, add this function to your shell config:

**Bash/Zsh** (`~/.bashrc` or `~/.zshrc`):
```bash
arbor() {
    command arbor "$@"
    local exit_code=$?

    if [ $exit_code -eq 0 ] && [ "$1" = "work" ] && [ -n "$2" ]; then
        # Extract branch name from arguments (first non-flag argument)
        local branch=""
        for arg in "${@:2}"; do
            if [[ ! "$arg" =~ ^- ]]; then
                branch="$arg"
                break
            fi
        done

        if [ -n "$branch" ]; then
            # Find the worktree path for this branch
            local worktree_path=$(git worktree list | grep "\\[$branch\\]" | awk '{print $1}')
            if [ -d "$worktree_path" ]; then
                cd "$worktree_path"
            fi
        fi
    fi

    return $exit_code
}
```

**Fish** (`~/.config/fish/functions/arbor.fish`):
```fish
function arbor
    command arbor $argv
    set exit_code $status

    if test $exit_code -eq 0; and test "$argv[1]" = "work"; and test (count $argv) -ge 2
        # Extract branch name (first non-flag argument)
        set branch ""
        for arg in $argv[2..-1]
            if not string match -q -- "-*" $arg
                set branch $arg
                break
            end
        end

        if test -n "$branch"
            # Find the worktree path for this branch
            set worktree_path (git worktree list | grep "\\[$branch\\]" | awk '{print $1}')
            if test -d "$worktree_path"
                cd "$worktree_path"
            end
        end
    end

    return $exit_code
end
```

After adding this function, reload your shell or run `source ~/.bashrc` (or equivalent).

## Quick Start

```bash
# Initialise a new Laravel project
arbor init git@github.com:user/my-laravel-app.git

# Initialise with database migrations
arbor init git@github.com:user/my-laravel-app.git --migrate=migrate:fresh

# Create a feature worktree
arbor work feature/user-auth

# Create a worktree from a specific base branch
arbor work feature/user-auth -b develop

# Create a worktree with migrations and copy .env from main branch
arbor work feature/user-auth --migrate=migrate --copy-env

# List all worktrees with their status
arbor list

# Remove a worktree when done
arbor remove feature/user-auth

# Clean up merged worktrees
arbor prune

# Destroy the entire project (removes worktrees and bare repo)
arbor destroy
```

## Command Options

### Migration Control

Both `arbor init` and `arbor work` support database migration control via the `--migrate` flag:

```bash
# Skip migrations (default)
arbor init git@github.com:user/repo.git
arbor work feature/new-feature

# Run standard migrations with seeding
arbor init git@github.com:user/repo.git --migrate=migrate
arbor work feature/new-feature --migrate=migrate

# Run fresh migrations (drops all tables first)
arbor init git@github.com:user/repo.git --migrate=migrate:fresh
arbor work feature/new-feature --migrate=migrate:fresh
```

**Options:**
- `none` (default) - Skip database migrations entirely
- `migrate` - Run `php artisan migrate --seed --no-interaction`
- `migrate:fresh` - Run `php artisan migrate:fresh --seed --no-interaction`

### Environment Configuration

When creating a new worktree with `arbor work`, you can copy the `.env` file from your main branch instead of using `.env.example`:

```bash
# Copy .env from main branch
arbor work feature/new-feature --copy-env

# Combine with migrations
arbor work feature/new-feature --migrate=migrate --copy-env
```

**Use cases:**
- Maintain consistent database credentials across worktrees
- Preserve API keys and third-party service configurations
- Quickly spin up worktrees with production-like settings

**Note:** By default (without `--copy-env`), worktrees use `.env.example` as the source.

## Documentation

See [AGENTS.md](./AGENTS.md) for development guide.

- Command reference
- Configuration files
- Scaffold presets
- Testing strategy

## License

MIT
