package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/spf13/pflag"
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
			log.Printf("Opened %q but stat failed with err: %v\n", absFilePath, err)
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
				log.Printf("File %q rotated because old file went away. newStatErr: %v\n", absFilePath, err)
				monitoredFiles[absFilePath] = currentFsInfo
				continue
			}

			recreated := !os.SameFile(currentStat, prevFileStat)
			truncated := currentStat.Size() < entry.size

			if recreated {
				log.Printf("File %q rotated due to recreation, stat of prev file: %+v, stat of current file: %+v\n", absFilePath, prevFileStat, currentStat)
			} else if truncated {
				log.Printf("File %q rotated due to truncation, lastSize: %d, currentSize: %d\n", absFilePath, entry.size, currentStat.Size())
			}

		}

		monitoredFiles[absFilePath] = currentFsInfo
	}
}

func main() {
	pflag.ErrHelp = fmt.Errorf("Scans the specified directories for files that have rotated (think logrotate)")
	dirGlobs := pflag.StringArrayP("directories", "d", []string{"/var/log/pods"}, "Directories to monitor. Will recursively look for any files, conceptually adds '**/*'.")
	scanPeriod := pflag.DurationP("period", "p", time.Second, "How long to sleep between scans (seconds)")

	pflag.Parse()

	log.Printf("Monitoring directories: %v", dirGlobs)
	monitoredFiles := make(map[string]fsInfo)
	for {
		for _, glob := range *dirGlobs {
			monitor(monitoredFiles, filepath.Join(glob, "/**/*"))
		}
		log.Printf("Finished scan, currently monitoring %d files.\n", len(monitoredFiles))
		time.Sleep(*scanPeriod)
	}
}
