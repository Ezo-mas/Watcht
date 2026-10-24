package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/Ezo-mas/watcht/internal/config"
	"github.com/Ezo-mas/watcht/internal/db"
)

func backup(args []string, stdout, stderr io.Writer) int {
	cfg, err := config.Load()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "watcht backup:", err)
		return 1
	}
	var dest string
	switch len(args) {
	case 0:
		dest = db.BackupPath(cfg.BackupDir(), "manual", time.Now())
	case 1:
		dest = args[0]
	default:
		_, _ = fmt.Fprintf(stderr, "watcht backup: too many arguments\n\n%s", usage)
		return 2
	}
	if err := runBackup(context.Background(), cfg.DatabasePath(), dest); err != nil {
		_, _ = fmt.Fprintln(stderr, "watcht backup:", err)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, dest)
	return 0
}

// runBackup snapshots the database at path into dest. It refuses to run
// when there's no database yet, rather than creating an empty one.
func runBackup(ctx context.Context, path, dest string) error {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("no database at %s (is WATCHT_DATA_DIR right?)", path)
	} else if err != nil {
		return err
	}
	database, err := db.Open(ctx, path)
	if err != nil {
		return err
	}
	return errors.Join(database.Backup(ctx, dest), database.Close())
}
