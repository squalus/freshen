package main

import (
	"fmt"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
	cp "github.com/otiai10/copy"
	"log"
	"os"
	"os/exec"
	"path"
)

func RunUpdateScript(scriptOutput string, config *UpdateScript, flakeRoot string) (UpdateResult, error) {
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return UpdateResult{}, fmt.Errorf("os.MkdirTemp: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()
	dotGitPath := path.Join(flakeRoot, ".git")
	copyOpts := cp.Options{
		Skip: func(srcinfo os.FileInfo, src, dest string) (bool, error) {
			return src == dotGitPath, nil
		},
	}
	if err := cp.Copy(flakeRoot, tmpDir, copyOpts); err != nil {
		return UpdateResult{}, fmt.Errorf("cp.Copy: %w", err)
	}
	worktree, err := prepareGit(tmpDir)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("prepareGit: %w", err)
	}
	executable := path.Join(scriptOutput, config.Command)
	cmd := exec.Cmd{
		Path:   executable,
		Dir:    tmpDir,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	if err := cmd.Run(); err != nil {
		return UpdateResult{}, fmt.Errorf("exec.Cmd: %w", err)
	}
	out, err := diffGit(worktree)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("diffGit: %w", err)
	}
	for _, changedPath := range out.getPathsChanged() {
		log.Printf("Copying updated file=%s", changedPath)
		if err := cp.Copy(path.Join(tmpDir, changedPath), path.Join(flakeRoot, changedPath)); err != nil {
			return UpdateResult{}, fmt.Errorf("cp.Copy %s: %w", changedPath, err)
		}
	}
	return out, nil
}

func prepareGit(root string) (*git.Worktree, error) {
	if err := deleteGit(root); err != nil {
		return nil, fmt.Errorf("deleteGit: %w", err)
	}
	fs := osfs.New(root)
	fscache := cache.NewObjectLRU(10000)
	storer := filesystem.NewStorage(fs, fscache)
	repo, err := git.Init(storer, fs)
	if err != nil {
		return nil, fmt.Errorf("git.Init: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("git.Worktree: %w", err)
	}
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return nil, fmt.Errorf("git.AddWithOptions: %w", err)
	}

	return wt, nil
}

func deleteGit(root string) error {
	gitRoot := path.Join(root, ".git")
	if err := os.RemoveAll(gitRoot); err != nil {
		return fmt.Errorf("os.RemoveAll: %w", err)
	}
	return nil
}

func diffGit(wt *git.Worktree) (UpdateResult, error) {
	status, err := wt.Status()
	if err != nil {
		return UpdateResult{}, fmt.Errorf("git.Status: %w", err)
	}
	out := NewUpdateResult()
	for filename, maybeFileStatus := range status {
		if maybeFileStatus == nil {
			continue
		}
		fileStatus := *maybeFileStatus
		if fileStatus.Worktree == git.Modified {
			out.addPath(filename)
		}
	}
	return out, nil
}
