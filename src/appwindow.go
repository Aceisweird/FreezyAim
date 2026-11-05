package main

import (
	"os"
	"path/filepath"
)

const windowSize = "1240,800"

// appWindowArgs are the browser options that give the app its own plain window.
func appWindowArgs(url string) ([]string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	return []string{
		"--app=" + url,
		"--window-size=" + windowSize,
		"--user-data-dir=" + filepath.Join(cacheDir, "FreezyAim", "window"),
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-sync",
		"--hide-crash-restore-bubble",
	}, nil
}
