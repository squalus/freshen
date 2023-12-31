package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// FreshenConfig is the top level for freshen.json
type FreshenConfig struct {
	UpdateTasks []UpdateTask `json:"update_tasks"`
}

// UpdateTask contains info for an auto update task
type UpdateTask struct {
	// Name of the update task
	Name string `json:"name"`
	// MainAttrPath for the main build
	MainAttrPath string `json:"attr_path"`
	// The flake inputs that the build uses
	Inputs []string `json:"inputs"`
	// Info about the derived hashes that need updating when the flake inputs update
	DerivedHashes []UpdateDerivedConfig `json:"derived_hashes"`
	// AttrPaths of update scripts to be run. Scripts will be executed with the flake root as the working directory.
	UpdateScripts []UpdateScript `json:"update_scripts"`
	// AttrPaths that will test the build
	Tests []TestConfig `json:"tests"`
	// Names of other required update tasks. These must all be updated successfully for the task to succeed
	RequiredUpdateTasks []string `json:"required_update_tasks"`
}

type UpdateScript struct {
	// AttrPath is the attr path of the update script
	AttrPath string `json:"attr_path"`
	// Executable is the file path of the command to execute, relative to the root of the script output in the Nix store
	Executable string `json:"executable"`
	// Arguments provided to the executable, if any
	Args []string `json:"args"`
}

// UpdateDerivedConfig describes the update tasks derived from a build
type UpdateDerivedConfig struct {
	// AttrPath that will produce a forced hash mismatch when built
	AttrPath string `json:"attr_path"`
	// Filename where the derived hash is stored as a JSON string. Relative to the flake root.
	Filename string `json:"filename"`
}

// TestConfig describes tests that will run to verify an update
type TestConfig struct {
	// AttrPath to build for the test. The test passes if the build succeeds.
	AttrPath string `json:"attr_path"`
	// DisableSandbox will turn off the Nix sandbox, e.g. for network access
	DisableSandbox bool `json:"disable_sandbox"`
}

// GitConfig is the configuration for a remote git task
type GitConfig struct {
	// Author name of the commit message
	Author string `json:"author"`
	// Email of the commit message author
	Email string `json:"email"`
	// Branch name to fetch and commit
	Branch string `json:"branch"`
	// GitHub specific configuration
	GitHub *GitHubConfig `json:"github"`
}

// GitHubConfig is the GitHub-specific configuration for a remote git task
type GitHubConfig struct {
	// Owner name of the repo
	Owner string `json:"owner"`
	// Repo name
	Repo string `json:"repo"`
	// Filename of a GitHub token. Default: ${CREDENTIALS_DIRECTORY}/github_token.txt
	TokenFile string `json:"token_file"`
}

func ReadJsonFile[T interface{}](path string) (*T, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out T
	if err := json.Unmarshal(buf, &out); err != nil {
		return nil, fmt.Errorf("json.Unmarshal %w", err)
	}
	return &out, nil
}
