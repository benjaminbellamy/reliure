package cli

import (
	"os"
	"path/filepath"
	"time"
)

func defaultSnapshotDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "reliure", "snapshots")
}

func defaultBackupDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents")
}

// defaultSnapshotPath returns ``~/.config/reliure/snapshots/reliure-YYYYMMDD.yaml``.
func defaultSnapshotPath() string {
	return filepath.Join(defaultSnapshotDir(), "reliure-"+time.Now().Format("20060102")+".yaml")
}
