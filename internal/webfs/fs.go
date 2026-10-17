package webfs

import (
	"io/fs"
	"os"
	"path/filepath"
)

// FS returns the web/ directory from the module root (plain HTML/CSS/JS, no build step).
func FS() (fs.FS, error) {
	root := filepath.Join(ModuleRoot(), "web")
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return nil, err
	}
	return os.DirFS(root), nil
}

func ModuleRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
		dir = parent
	}
}
