package errors

import "errors"

var (
	ErrWorktreeNotFound   = errors.New("worktree not found")
	ErrConfigNotFound     = errors.New("configuration not found")
	ErrGitOperationFailed = errors.New("git operation failed")
)
