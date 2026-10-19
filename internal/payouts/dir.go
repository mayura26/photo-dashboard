package payouts

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const Ext = ".jpg"

var idRe = regexp.MustCompile(`^[a-f0-9]{32}$`)

// Dir holds payout screenshots beside the SQLite DB.
type Dir struct {
	Path string
}

// DirFromDBPath returns data/payouts beside hq.db (e.g. /data/hq.db → /data/payouts).
func DirFromDBPath(dbPath string) Dir {
	return Dir{Path: filepath.Join(filepath.Dir(dbPath), "payouts")}
}

// Ensure creates the payouts directory if needed.
func (d Dir) Ensure() error {
	if d.Path == "" {
		return fmt.Errorf("payouts: empty dir")
	}
	return os.MkdirAll(d.Path, 0o755)
}

// NewID returns a 32-char hex id.
func NewID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// ValidID reports whether id is a safe filename stem.
func ValidID(id string) bool {
	return idRe.MatchString(strings.ToLower(id))
}

// FilePath returns the absolute path for a payout screenshot.
func (d Dir) FilePath(id string) string {
	return filepath.Join(d.Path, strings.ToLower(id)+Ext)
}

// WriteAtomic writes JPEG bytes for id.
func (d Dir) WriteAtomic(id string, jpeg []byte) error {
	if !ValidID(id) {
		return fmt.Errorf("payouts: invalid id")
	}
	if err := d.Ensure(); err != nil {
		return err
	}
	tmp := d.FilePath(id) + ".tmp"
	final := d.FilePath(id)
	if err := os.WriteFile(tmp, jpeg, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// Remove deletes the file for id (ok if missing).
func (d Dir) Remove(id string) error {
	if !ValidID(id) {
		return fmt.Errorf("payouts: invalid id")
	}
	err := os.Remove(d.FilePath(id))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
