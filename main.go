package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/alecthomas/kong"
	"github.com/squalus/freshen/flake"
	"log"
	"os"
	"path"
)

type globals struct {
}

type Cli struct {
	globals
	Update       updateCmd       `cmd:"" help:"Run local update task"`
	RemoteUpdate RemoteUpdateCmd `cmd:"" help:"Run remote update task"`
}

func main() {
	var cli Cli
	ctx := kong.Parse(&cli)
	err := ctx.Run(&cli.globals)
	ctx.FatalIfErrorf(err)
}

type updateCmd struct {
	Name     string `help:"Name of update task to run" required:""`
	RepoPath string `name:"repo-path" help:"Path of repository root" type:"path"`
	Check    bool   `help:"Always run all build and test steps (even if no inputs changed)"`
}

func (u *updateCmd) Run() error {
	if u.RepoPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		u.RepoPath = cwd
	}
	log.Printf("repoPath=%s", u.RepoPath)
	if err := validateRepoPath(u.RepoPath); err != nil {
		return fmt.Errorf("validateRepoPath %w", err)
	}
	configFilePath := path.Join(u.RepoPath, "freshen.json")
	autoUpdateConfig, err := ReadJsonFile[FreshenConfig](configFilePath)
	if err != nil {
		return fmt.Errorf("ReadAutoUpdateConfig %w", err)
	}

	updateFlake := flake.Flake{Path: u.RepoPath}
	autoUpdate := NewUpdateSpec(autoUpdateConfig, updateFlake)

	_, err = autoUpdate.RunUpdateName(u.Name, u.Check)
	return err
}

type RemoteUpdateCmd struct {
	Name   string `help:"Name of update task to run" required:""`
	Config string `help:"Path to git config file" required:""`
}

func (u *RemoteUpdateCmd) Run() error {
	var gc GitConfig
	b, err := os.ReadFile(u.Config)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &gc); err != nil {
		return err
	}
	taskRunner, err := NewGitHubTaskRunner(&gc)
	if err != nil {
		return err
	}
	if err = taskRunner.runGitTask(context.Background(), u.Name); err != nil {
		return err
	}
	return nil
}

func validateRepoPath(repoPath string) error {
	_, err := os.Stat(path.Join(repoPath, "flake.nix"))
	if err != nil {
		return fmt.Errorf("check repo path. repoPath=%s does not look valid: %w", repoPath, err)
	}
	return nil
}
