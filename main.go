package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/spf13/pflag"
	"golang.org/x/exp/slog"
)

type fsInfo struct {
	osFile *os.File
	size   int64
}

// This file rotation detection logic aims to mirror
// https://github.com/DataDog/datadog-agent/blob/main/pkg/logs/internal/tailers/file/rotate_nix.go
func monitor(monitoredFiles map[string]fsInfo, dir string) {
	basepath, pattern := doublestar.SplitPattern(dir)
	fsys := os.DirFS(basepath)

	res, err := doublestar.Glob(fsys, pattern)
	if err != nil {
		log.Println(err)
		return
	}
	for _, relFilePath := range res {
		absFilePath := filepath.Join(basepath, relFilePath)
		entry, seenFileBefore := monitoredFiles[absFilePath]

		currentOsFile, err := os.Open(absFilePath)
		if err != nil {
			// Found file in glob, but couldn't open a reference to it.
			// Maybe permission error? Too spammy to log all of them, just skip.
			continue
		}

		currentStat, err := currentOsFile.Stat()
		if err != nil {
			// Opened file but couldn't stat the file, odd. Log as this is unusual
			slog.Warn("Opened file but stat failed with err.", "FileName", absFilePath, "Err", err)
			continue
		}
		if currentStat.IsDir() {
			// Don't care about directories
			continue
		}

		currentFsInfo := fsInfo{
			osFile: currentOsFile,
			size:   currentStat.Size(),
		}

		if seenFileBefore {
			prevFileStat, err := entry.osFile.Stat()

			oldFileWentAway := err != nil
			if oldFileWentAway {
				slog.Warn("File rotated because old file went away", "FileName", absFilePath, "NewStatErr", err)
				monitoredFiles[absFilePath] = currentFsInfo
				continue
			}

			recreated := !os.SameFile(currentStat, prevFileStat)
			truncated := currentStat.Size() < entry.size

			if recreated {
				slog.Warn("File rotated due to recreation", "FileName", absFilePath, "PrevStatResult", fmt.Sprintf("%+v", prevFileStat), "CurrentStatResult", fmt.Sprintf("%+v", currentStat))
			} else if truncated {
				slog.Warn("File rotated due to truncation", "FileName", absFilePath, "LastSize", entry.size, "CurrentSize", currentStat.Size())
			}

		}

		monitoredFiles[absFilePath] = currentFsInfo
	}
}

func logArgToSlogLevel(level string) slog.Level {
	switch level {
	case "info":
		return slog.LevelInfo
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func main() {
	pflag.ErrHelp = fmt.Errorf("Watches the specified directories for files that have rotated (think logrotate)")
	dirGlobs := pflag.StringArrayP("directories", "d", []string{"/var/log/pods"}, "Directories to monitor. Will recursively look for any files, conceptually adds '**/*'.")
	scanPeriod := pflag.DurationP("period", "p", time.Second, "How long to sleep between scans (seconds)")
	logLevel := pflag.StringP("logLevel", "l", "", "DEBUG | INFO | WARN | ERROR")

	pflag.Parse()

	opts := slog.HandlerOptions{
		Level: logArgToSlogLevel(strings.ToLower(*logLevel)),
	}
	logger := slog.New(opts.NewTextHandler(os.Stdout))
	slog.SetDefault(logger)

	slog.Info("Watching the following directories", "MonitoredGlobs", dirGlobs)
	monitoredFiles := make(map[string]fsInfo)
	for {
		for _, glob := range *dirGlobs {
			monitor(monitoredFiles, filepath.Join(glob, "/**/*"))
		}
		slog.Debug("Finished scan.", "MonitoredFiles", len(monitoredFiles))
		time.Sleep(*scanPeriod)
	}
}
