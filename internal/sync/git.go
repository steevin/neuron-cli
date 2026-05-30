package gitsync

import (
	"fmt"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// Syncer handles git synchronization for the vault.
type Syncer struct {
	VaultPath string
	Remote    string
	AuthToken string
}

// NewSyncer creates a new Syncer.
func NewSyncer(vaultPath, remote string) *Syncer {
	return &Syncer{
		VaultPath: vaultPath,
		Remote:    remote,
	}
}

// SyncResult holds the result of a sync operation.
type SyncResult struct {
	FilesChanged int
	CommitHash   string
	Pushed       bool
	Message      string
}

// initRepo ensures the vault is a git repository.
func (s *Syncer) initRepo() (*git.Repository, error) {
	repo, err := git.PlainOpen(s.VaultPath)
	if err == git.ErrRepositoryNotExists {
		return git.PlainInit(s.VaultPath, false)
	}
	return repo, err
}

// Sync commits local changes and optionally pushes them to the remote.
func (s *Syncer) Sync() (*SyncResult, error) {
	repo, err := s.initRepo()
	if err != nil {
		return nil, fmt.Errorf("failed to open repo: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %v", err)
	}

	// Add all files
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return nil, fmt.Errorf("git add failed: %v", err)
	}

	status, err := wt.Status()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %v", err)
	}

	changed := 0
	for _, fileStatus := range status {
		if fileStatus.Worktree != git.Unmodified || fileStatus.Staging != git.Unmodified {
			changed++
		}
	}

	if changed == 0 {
		// Nothing to commit, but we might still need to push if ahead of remote
		return &SyncResult{FilesChanged: 0, Message: "Everything up to date"}, nil
	}

	commitMsg := fmt.Sprintf("neuron: sync %s (%d files changed)", time.Now().Format("2006-01-02 15:04"), changed)
	hash, err := wt.Commit(commitMsg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "neuron-cli",
			Email: "neuron@local",
			When:  time.Now(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("commit failed: %v", err)
	}

	res := &SyncResult{
		FilesChanged: changed,
		CommitHash:   hash.String(),
		Message:      commitMsg,
	}

	if s.Remote != "" {
		// Ensure remote exists
		_, err := repo.Remote("origin")
		if err == git.ErrRemoteNotFound {
			_, err = repo.CreateRemote(&config.RemoteConfig{
				Name: "origin",
				URLs: []string{s.Remote},
			})
			if err != nil {
				return res, fmt.Errorf("failed to create remote: %v", err)
			}
		}

		pushOpts := &git.PushOptions{}
		if s.AuthToken != "" {
			pushOpts.Auth = &http.BasicAuth{
				Username: "token", // Git providers usually ignore this when using a PAT
				Password: s.AuthToken,
			}
		}

		err = repo.Push(pushOpts)
		if err != nil && err != git.ErrNonFastForwardUpdate && err != git.NoErrAlreadyUpToDate {
			return res, fmt.Errorf("push failed: %v", err)
		}
		if err == nil {
			res.Pushed = true
		}
	}

	return res, nil
}

// Pull fetches and merges remote changes.
func (s *Syncer) Pull() error {
	if s.Remote == "" {
		return fmt.Errorf("no remote configured")
	}
	repo, err := s.initRepo()
	if err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	pullOpts := &git.PullOptions{RemoteName: "origin"}
	if s.AuthToken != "" {
		pullOpts.Auth = &http.BasicAuth{
			Username: "token",
			Password: s.AuthToken,
		}
	}
	err = wt.Pull(pullOpts)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}
	return nil
}

// Status returns a list of changed file paths.
func (s *Syncer) Status() ([]string, error) {
	repo, err := s.initRepo()
	if err != nil {
		return nil, err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, err
	}
	status, err := wt.Status()
	if err != nil {
		return nil, err
	}
	
	var changed []string
	for path, fileStatus := range status {
		if fileStatus.Worktree != git.Unmodified || fileStatus.Staging != git.Unmodified {
			changed = append(changed, path)
		}
	}
	return changed, nil
}
