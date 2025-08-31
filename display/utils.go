package display

import (
	"os"
	"path/filepath"
	"sort"
	"wecker/config"
)

// getAvailableFiles returns available files for the given alarm source
func getAvailableFiles(source config.AlarmSource, cfg *config.Config) []string {
	var files []string
	var searchDir string

	switch source {
	case config.SourceBuzzer:
		searchDir = cfg.BuzzerDir
	case config.SourceSoother:
		searchDir = cfg.SootherDir
	default:
		return files
	}

	if entries, err := os.ReadDir(searchDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".tone" {
				files = append(files, entry.Name())
			}
		}
	}

	sort.Strings(files)
	return files
}
