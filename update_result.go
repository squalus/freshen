package main

type UpdateResult struct {
	// changed paths, relative to repo root
	pathsChanged map[string]struct{}
}

func NewUpdateResult() UpdateResult {
	return UpdateResult{
		pathsChanged: make(map[string]struct{}),
	}
}

func (u *UpdateResult) union(other UpdateResult) {
	for pathChanged, _ := range other.pathsChanged {
		u.addPath(pathChanged)
	}
}

func (u *UpdateResult) addPath(path string) {
	u.pathsChanged[path] = struct{}{}
}

func (u *UpdateResult) addPaths(paths []string) {
	for _, pathChanged := range paths {
		u.addPath(pathChanged)
	}
}

func (u *UpdateResult) getPathsChanged() []string {
	out := make([]string, 0, len(u.pathsChanged))
	for k, _ := range u.pathsChanged {
		out = append(out, k)
	}
	return out
}

func (u *UpdateResult) empty() bool {
	return len(u.pathsChanged) == 0
}
