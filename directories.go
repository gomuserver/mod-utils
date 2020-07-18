package gomu

import (
	"os/exec"
	"path"
	"strings"

	"github.com/gomuserver/mod-utils/sort"
)

// PopulateLibsFromTargets will aggregate all libs within all target dirs
func (mu *MU) PopulateLibsFromTargets() {
	libs := make(sort.StringArray, 0)
	for index := range mu.Options.TargetDirectories {
		libs = append(libs, GetLibsInDirectory(mu.Options.TargetDirectories[index])...)
	}

	mu.AllDirectories = libs
	return
}

// GetLibsInDirectory returns all libs a given directory
func GetLibsInDirectory(dir string) (libs sort.StringArray) {
	cmd := exec.Command("ls")
	if len(dir) > 0 {
		cmd.Dir = dir
	}
	stdout, err := cmd.Output()

	if err != nil {
		return
	}

	// Parse files from exec "ls"
	libs = strings.Split(string(stdout), "\n")
	for index := range libs {
		switch libs[index] {
		case ".", "..", dir:
			// Ignore non-repositories
		default:
			libs[index] = path.Join(dir, libs[index])
		}
	}

	return
}
