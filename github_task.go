package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/google/go-github/v48/github"
	"github.com/squalus/freshen/flake"
	"golang.org/x/net/context/ctxhttp"
	"golang.org/x/oauth2"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

type GitHubTaskRunner struct {
	Config      *GitConfig
	tokenSource oauth2.TokenSource
}

func NewGitHubTaskRunner(config *GitConfig) (*GitHubTaskRunner, error) {
	var out GitHubTaskRunner
	out.Config = config
	tokenPath := config.GitHub.TokenFile
	if config.GitHub.TokenFile == "" {
		credsDir := os.Getenv("CREDENTIALS_DIRECTORY")
		if credsDir == "" {
			return nil, fmt.Errorf("blank CREDENTIALS_DIRECTORY env var. create it or provide GitHub.TokenFile")
		}
		tokenPath = path.Join(credsDir, "github_token.txt")
	}
	tokenBuf, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("github token os.ReadFile %w", err)
	}
	token := strings.TrimSpace(string(tokenBuf))
	out.tokenSource = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	return &out, nil
}

func (g *GitHubTaskRunner) githubClient(ctx context.Context) *github.Client {
	oauthClient := oauth2.NewClient(ctx, g.tokenSource)
	return github.NewClient(oauthClient)
}

func (g *GitHubTaskRunner) latestCommitHash(ctx context.Context, branchName string) (string, error) {
	ghClient := g.githubClient(ctx)
	branch, _, err := ghClient.Repositories.GetBranch(ctx, g.Config.GitHub.Owner, g.Config.GitHub.Repo, branchName, true)
	if err != nil {
		return "", fmt.Errorf("github.Repositories.GetBranch %w", err)
	}
	if branch.GetCommit() == nil || branch.GetCommit().GetSHA() == "" {
		return "", fmt.Errorf("nil hash")
	}
	return branch.GetCommit().GetSHA(), nil
}

func (g *GitHubTaskRunner) createBlob(ctx context.Context, path string) (hash string, err error) {
	ghClient := g.githubClient(ctx)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	base64Buf := base64.StdEncoding.EncodeToString(b)
	encoding := "base64"
	blob := github.Blob{
		Content:  &base64Buf,
		Encoding: &encoding,
	}
	outBlob, _, err := ghClient.Git.CreateBlob(ctx, g.Config.GitHub.Owner, g.Config.GitHub.Repo, &blob)
	if err != nil || outBlob.GetSHA() == "" {
		return "", fmt.Errorf("github.Git.CreateBlob %w", err)
	}
	return outBlob.GetSHA(), nil
}

func (g *GitHubTaskRunner) createTree(ctx context.Context, rootDir string, relativePaths []string, baseHash string) (hash string, err error) {
	entries := make([]*github.TreeEntry, 0, len(relativePaths))
	blobType := "blob"
	mode := "100644"
	for i, curPath := range relativePaths {
		log.Printf("creating blob filename=%s", curPath)
		blobHash, err := g.createBlob(ctx, path.Join(rootDir, curPath))
		if err != nil {
			return "", fmt.Errorf("createBlob: %w", err)
		}
		entries = append(entries, &github.TreeEntry{
			SHA:  &blobHash,
			Path: &relativePaths[i],
			Mode: &mode,
			Type: &blobType,
		})
	}
	ghClient := g.githubClient(ctx)
	log.Printf("creating tree")
	tree, _, err := ghClient.Git.CreateTree(ctx, g.Config.GitHub.Owner, g.Config.GitHub.Repo, baseHash, entries)
	if err != nil {
		return "", fmt.Errorf("github.Git.CreateTree %w", err)
	}
	return tree.GetSHA(), nil
}

func (g *GitHubTaskRunner) commit(ctx context.Context, latestHash string, treeHash string, author *github.CommitAuthor, message string) (hash string, err error) {
	ghClient := g.githubClient(ctx)
	commit, _, err := ghClient.Git.CreateCommit(ctx, g.Config.GitHub.Owner, g.Config.GitHub.Repo, &github.Commit{
		Author:  author,
		Message: &message,
		Tree:    &github.Tree{SHA: &treeHash},
		Parents: []*github.Commit{{SHA: &latestHash}},
	})
	if err != nil {
		return "", fmt.Errorf("github.Git.Commit: %w", err)
	}
	return commit.GetSHA(), nil
}

func (g *GitHubTaskRunner) updateBranch(ctx context.Context, branch, newHash string) error {
	ghClient := g.githubClient(ctx)
	ref := fmt.Sprintf("refs/heads/%s", branch)
	reference := &github.Reference{
		Ref:    &ref,
		Object: &github.GitObject{SHA: &newHash}}
	_, _, err := ghClient.Git.UpdateRef(ctx, g.Config.GitHub.Owner, g.Config.GitHub.Repo, reference, false)
	if err != nil {
		return fmt.Errorf("github.Git.UpdateRef: %w", err)
	}
	return nil
}

func (g *GitHubTaskRunner) downloadGitHubRepo(ctx context.Context, branch, toDir string) error {
	ghClient := g.githubClient(ctx)
	url, _, err := ghClient.Repositories.GetArchiveLink(ctx, g.Config.GitHub.Owner, g.Config.GitHub.Repo, github.Tarball, &github.RepositoryContentGetOptions{Ref: branch}, true)
	if err != nil {
		return fmt.Errorf("github.Repositories.GetArchiveLink %w", err)
	}
	resp, err := ctxhttp.Get(ctx, http.DefaultClient, url.String())
	if err != nil {
		return fmt.Errorf("ctxhttp.Get %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("repo archive get: %w", err)
	}
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip.NewReader %w", err)
	}
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return fmt.Errorf("tar: %w", err)
		case header == nil:
			return errors.New("tar header nil")
		}
		if strings.Contains(header.Name, "..") {
			return fmt.Errorf("bad filename: %s", header.Name)
		}
		header.Name = strings.TrimSuffix(header.Name, "/")
		sp := strings.Split(header.Name, "/")
		if len(sp) <= 1 {
			continue
		}
		strippedName := strings.Join(sp[1:], "/")
		target := path.Join(toDir, strippedName)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(target, 0700); err != nil {
				return fmt.Errorf("mkdir %w", err)
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("os.OpenFile: %w", err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return fmt.Errorf("io.Copy: %w", err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("file.Close: %w", err)
			}
		case tar.TypeSymlink:
			return fmt.Errorf("unimplemented tar record: %d", header.Typeflag)
		}
	}
}

func (g *GitHubTaskRunner) runGitTask(ctx context.Context, name string) error {
	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		return fmt.Errorf("os.MkdirTemp: %w", err)
	}
	log.Printf("tempDir=%s", tempDir)

	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	log.Printf("Downloading repository")
	if err := g.downloadGitHubRepo(ctx, g.Config.Branch, tempDir); err != nil {
		return err
	}

	freshenConfig, err := ReadJsonFile[FreshenConfig](path.Join(tempDir, "freshen.json"))
	if err != nil {
		return fmt.Errorf("ReadAutoUpdateConfig: %w", err)
	}
	updateFlake := flake.Flake{Path: tempDir}
	au := NewUpdateSpec(freshenConfig, updateFlake)

	latestHash, err := g.latestCommitHash(ctx, g.Config.Branch)
	if err != nil {
		return fmt.Errorf("latestCommitHash: %w", err)
	}
	log.Printf("latestCommitHash=%s", latestHash)

	result, err := au.RunUpdateName(name, false)
	if err != nil {
		return fmt.Errorf("runUpdateName: %w", err)
	}

	if result.empty() {
		log.Printf("no update changes")
		return nil
	}

	for pathChanged := range result.pathsChanged {
		log.Printf("changed file: %s", pathChanged)
	}

	treeHash, err := g.createTree(ctx, au.Flake.Path, result.getPathsChanged(), latestHash)
	if err != nil {
		return fmt.Errorf("createTree: %w", err)
	}

	date := time.Now()
	author := &github.CommitAuthor{
		Date:  &date,
		Name:  &g.Config.Author,
		Email: &g.Config.Email,
	}
	message := fmt.Sprintf("%s: update", name)

	log.Printf("committing")
	commitHash, err := g.commit(ctx, latestHash, treeHash, author, message)
	if err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	log.Printf("commitHash=%s", commitHash)

	if err := g.updateBranch(ctx, g.Config.Branch, commitHash); err != nil {
		return fmt.Errorf("updateBranch: %w", err)
	}
	log.Printf("branch=%s updated", g.Config.Branch)

	return nil
}
