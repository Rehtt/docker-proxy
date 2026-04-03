package cache

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

func RunCleaner(ctx context.Context, dir string, ttl time.Duration, interval time.Duration) {
	if dir == "" || ttl <= 0 {
		return
	}
	if interval <= 0 {
		interval = time.Hour
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		sweep(dir, ttl)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweep(dir, ttl)
			}
		}
	}()
}

func sweep(dir string, ttl time.Duration) {
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Base(path) != "meta" {
			return nil
		}
		st, err := d.Info()
		if err != nil {
			return nil
		}
		if time.Since(st.ModTime()) <= ttl {
			return nil
		}
		_ = os.RemoveAll(filepath.Dir(path))
		return nil
	})
}
