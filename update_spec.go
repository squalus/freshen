package main

import (
	"encoding/json"
	"fmt"
	"github.com/squalus/freshen/flake"
	"log"
	"os"
	"path"
	"strings"
)

type UpdateSpec struct {
	Flake        flake.Flake
	Config       *FreshenConfig
	nameToConfig map[string]*UpdateTask
}

func NewUpdateSpec(config *FreshenConfig, flake flake.Flake) *UpdateSpec {
	var out UpdateSpec
	out.Flake = flake
	out.Config = config
	out.nameToConfig = make(map[string]*UpdateTask, len(out.Config.UpdateTasks))
	for i, curConfig := range config.UpdateTasks {
		out.nameToConfig[curConfig.Name] = &config.UpdateTasks[i]
	}
	return &out
}

type RunMode string

const RunModeOnFlakeInputChange RunMode = "on_flake_input_change"
const RunModeAlways RunMode = "always"

func (a *UpdateSpec) RunUpdateName(name string, check bool) (UpdateResult, error) {
	config, ok := a.nameToConfig[name]
	if !ok {
		var names []string
		for _, task := range a.Config.UpdateTasks {
			names = append(names, task.Name)
		}
		nameMsg := strings.Join(names, ",")
		return UpdateResult{}, fmt.Errorf("no update config with name=%s validNames=%s", name, nameMsg)
	}
	log.Printf("name=%s running linked updates", config.Name)
	out := NewUpdateResult()
	for _, linkedUpdate := range config.RequiredUpdateTasks {
		if config.Name == linkedUpdate {
			return UpdateResult{}, fmt.Errorf("name=%s references self in linkedUpdates", config.Name)
		}
		result, err := a.RunUpdateName(linkedUpdate, check)
		if err != nil {
			return UpdateResult{}, fmt.Errorf("linkedUpdate=%s %w", linkedUpdate, err)
		}
		out.union(result)
	}

	oldLocks, err := a.Flake.MetadataLocks()
	if err != nil {
		return UpdateResult{}, fmt.Errorf("flake.MetadataLocks %w", err)
	}

	log.Printf("name=%s updating inputs", config.Name)

	var anyInputChanged bool
	for _, inputName := range config.Inputs {
		result, err := a.updateInput(inputName, oldLocks)
		if err != nil {
			return UpdateResult{}, fmt.Errorf("updateInput name=%s inputName=%s %w", config.Name, inputName, err)
		}
		if result == nil {
			log.Printf("name=%s inputName=%s: no input change", config.Name, inputName)
			continue
		}
		log.Printf("name=%s inputName=%s %s -> %s", config.Name, inputName, result.old, result.new)
		anyInputChanged = true
	}

	var derivedHashes []UpdateDerivedConfig
	var updateScripts []UpdateScript

	if anyInputChanged {
		out.addPath("flake.lock")
		derivedHashes = config.DerivedHashes
		updateScripts = config.UpdateScripts
	} else {
		log.Printf("name=%s: no inputs changed", config.Name)
		for _, derivedHash := range config.DerivedHashes {
			if derivedHash.RunMode == string(RunModeAlways) {
				derivedHashes = append(derivedHashes, derivedHash)
			}
		}
		for _, updateScript := range config.UpdateScripts {
			if updateScript.RunMode == string(RunModeAlways) {
				updateScripts = append(updateScripts, updateScript)
			}
		}
	}

	continueRunning := !out.empty() || check || (len(derivedHashes) > 0 || len(updateScripts) > 0)
	if !continueRunning {
		return UpdateResult{}, nil
	}

	log.Printf("name=%s updating derived hashes", config.Name)
	if len(derivedHashes) > 0 {
		derivedHashesResult, err := a.updateDerivedHashes(config)
		if err != nil {
			return UpdateResult{}, fmt.Errorf("updateDerivedHash: attrPath=%s %w", config.MainAttrPath, err)
		}
		if derivedHashesResult.empty() {
			log.Printf("name=%s no derived attrPath changed", config.Name)
		}
		out.union(derivedHashesResult)
	}

	log.Printf("name=%s running update scripts", config.Name)
	if len(updateScripts) > 0 {
		updateScriptResult, err := a.runUpdateScripts(config)
		if err != nil {
			return UpdateResult{}, fmt.Errorf("updateScriptResult: attrPath=%s %w", config.MainAttrPath, err)
		}
		if updateScriptResult.empty() {
			log.Printf("name=%s updateScript did not change any files", config.Name)
		}
		out.union(updateScriptResult)
	}

	if out.empty() && !check {
		return UpdateResult{}, nil
	}

	if config.MainAttrPath == "" {
		log.Printf("name=%s no main derivation", config.Name)
	} else {
		log.Printf("name=%s building main derivation", config.Name)
		if _, _, err = a.Flake.BuildWithRawOutput(config.MainAttrPath, true); err != nil {
			return UpdateResult{}, fmt.Errorf("name=%s main derivation build failed %w", config.Name, err)
		}
	}

	log.Printf("name=%s building tests", config.Name)
	for _, testConfig := range config.Tests {
		log.Printf("name=%s building test attrPath=%s", config.Name, testConfig.AttrPath)
		if _, _, err = a.Flake.BuildWithRawOutput(testConfig.AttrPath, !testConfig.DisableSandbox); err != nil {
			return UpdateResult{}, fmt.Errorf("name=%s testAttrPath=%s test failed %w", config.Name, testConfig.AttrPath, err)
		}
	}
	return out, nil
}

func (a *UpdateSpec) updateDerivedHashes(config *UpdateTask) (UpdateResult, error) {
	out := NewUpdateResult()
	for _, derivedConfig := range config.DerivedHashes {
		result, err := a.updatedDerivedHash(derivedConfig)
		if err != nil {
			return NewUpdateResult(), fmt.Errorf("updateDerivedHash: %w", err)
		}
		if result == nil {
			log.Printf("name=%s derivedAttrPath=%s no change", config.Name, derivedConfig.AttrPath)
			continue
		}
		log.Printf("name=%s derivedAttrPath=%s %s -> %s", config.Name, derivedConfig.AttrPath, result.old, result.new)
		out.addPaths(result.pathsChanged)
	}
	return out, nil
}

func (a *UpdateSpec) runUpdateScripts(config *UpdateTask) (UpdateResult, error) {
	out := NewUpdateResult()
	for _, updateScript := range config.UpdateScripts {
		log.Printf("name=%s running update script attrPath=%s executable=%s args=%s", config.Name, updateScript.AttrPath, updateScript.Executable, updateScript.Args)
		scriptOutput, err := a.Flake.Build(updateScript.AttrPath)
		if err != nil {
			return NewUpdateResult(), fmt.Errorf("flake.Build attrPath=%s: %w", updateScript.AttrPath, err)
		}
		scriptOut, err := RunUpdateScript(scriptOutput, &updateScript, a.Flake.Path)
		if err != nil {
			return NewUpdateResult(), fmt.Errorf("RunUpdateScript: %w", err)
		}
		out.union(scriptOut)
	}
	return out, nil
}

type UpdateInputResult struct {
	old, new     string
	pathsChanged []string
}

func (a *UpdateSpec) updateInput(name string, oldLocks flake.Locks) (*UpdateInputResult, error) {
	oldRev, ok := oldLocks.InputRev(name)
	if !ok {
		return nil, fmt.Errorf("missing input in lock file: %s", name)
	}
	var out UpdateInputResult
	out.old = oldRev
	if err := a.Flake.UpdateInput(name); err != nil {
		return nil, fmt.Errorf("flake.UpdateInput: %w", err)
	}
	newLocks, err := a.Flake.MetadataLocks()
	if err != nil {
		return nil, fmt.Errorf("read lock file: %w", err)
	}
	out.new, ok = newLocks.InputRev(name)
	if !ok {
		return nil, fmt.Errorf("missing input in lock file: %s", name)
	}

	if oldRev == out.new {
		return nil, nil
	}
	return &out, nil
}

func (a *UpdateSpec) updatedDerivedHash(config UpdateDerivedConfig) (*UpdateInputResult, error) {
	_, stderr, err := a.Flake.BuildWithRawOutput(config.AttrPath, true)
	if err == nil {
		return nil, fmt.Errorf("build unexpectedly succeeded")
	}

	hashMismatchResult, err := FindHashMismatch(stderr)
	if err != nil {
		return nil, fmt.Errorf("findHashMismatchResult %w", err)
	}
	var out UpdateInputResult
	out.new = hashMismatchResult.Got

	hashFilePath := path.Join(a.Flake.Path, config.Filename)
	out.old, err = readJsonStringFile(hashFilePath)
	if err != nil {
		return nil, fmt.Errorf("readJsonStringFile hashFilePath=%s %w", config.Filename, err)
	}

	if out.old == out.new {
		return nil, nil
	}

	if err := writeJsonStringFile(hashMismatchResult.Got, hashFilePath); err != nil {
		return nil, fmt.Errorf("writeJsonStringFile hashFilePath=%s %w", hashFilePath, err)
	}
	out.pathsChanged = []string{config.Filename}
	return &out, nil
}

func readJsonStringFile(path string) (out string, err error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if err = json.Unmarshal(buf, &out); err != nil {
		return "", err
	}
	return out, nil
}

func writeJsonStringFile(val, path string) error {
	buf, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0666)
}
