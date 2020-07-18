package gomu

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/gomuserver/mod-utils/sort"
)

// CleanModCache calls go clean --modcache from calling directory. No context necessary
func CleanModCache() error {
	cmd := exec.Command("go", "clean", "--modcache")
	return cmd.Run()
}

// ModInit calls go mod init on a given lib
func (lib *Library) ModInit() error {
	return lib.File.RunCmd("go", "mod", "init")
}

// ModTidy calls go mod tidy on a given lib
func (lib *Library) ModTidy() error {
	return lib.File.RunCmd("go", "mod", "tidy")
}

// ModClearFiles calls rm go.mod and rm go.sum, returning the success of both commands
func (lib *Library) ModClearFiles() (hasModFile, hasSumFile bool) {
	if lib.File.RunCmd("rm", "go.mod") == nil {
		hasModFile = true
	}

	if lib.File.RunCmd("rm", "go.sum") == nil {
		hasSumFile = true
	}

	return
}

// ModAddDeps adds a dep@version to go.mod to force-update or force-downgrade any deps in the filtered chain
func (lib *Library) ModAddDeps(listHead *sort.FileNode, shouldForce bool) {
	for itr := listHead; itr != nil && itr.File.Path != lib.File.Path; itr = itr.Next {
		// File has update if it was changed or needs to be explicitly set
		var hasUpdate = (itr.File.Updated || itr.File.Tagged || itr.File.Committed || len(itr.File.Version) != 0)

		// File should update if forced and direct or indirect, or if has update and direct
		if (shouldForce && lib.File.DependsOn(itr.File)) || (hasUpdate && lib.File.DirectlyImports(itr.File)) {
			// Create new node to add to independent list on lib with same file ref
			var node sort.FileNode
			node.File = itr.File
			lib.AddDep(&node)
		}
	}
}

// ModSetDeps adds a dep@version to go.mod to force-update or force-downgrade any deps in the filtered chain
func (lib *Library) ModSetDeps() (err error) {
	// Iterate through dep chain
	for itr := lib.updatedDeps; itr != nil; itr = itr.Next {
		if len(itr.File.Version) == 0 {
			tempLib := Library{}
			tempLib.File = itr.File
			itr.File.Version = tempLib.GetLatestTag()
		}

		url := itr.File.GetGoURL()

		// Get dep @ version (-d avoids building)
		if lib.File.RunCmd("go", "get", "-d", url+"@"+itr.File.Version) == nil {
			if itr.File.Updated || itr.File.Tagged || itr.File.Committed {
				lib.File.Output("Updated " + url + " @ " + itr.File.Version)
			} else {
				lib.File.Output("Set " + url + " @ " + itr.File.Version)
			}
		} else {
			lib.File.Output("Error: Failed to get " + url + " @ " + itr.File.Version)
			err = fmt.Errorf("Unable to set dependency: " + url + " @ " + itr.File.Version)
		}
	}
	return
}

// ModReplaceLocalFor adds replace clause for provided file
func (lib *Library) ModReplaceLocalFor(file sort.FileNode) (updated bool) {
	lib.File.Output("Replacing " + file.File.GetGoURL() + "...")
	return lib.AppendToModfile("\nreplace " + file.File.GetGoURL() + " => " + file.File.AbsPath() + "\n")
}

// ModReplaceLocal adds replace clause for all updated deps
func (lib *Library) ModReplaceLocal() (updated bool) {
	localSuffix := ""
	for fileItr := lib.updatedDeps; fileItr != nil; fileItr = fileItr.Next {
		lib.File.Output("Replacing " + fileItr.File.GetGoURL() + "...")
		localSuffix += "replace " + fileItr.File.GetGoURL() + " => " + fileItr.File.AbsPath() + "\n"
	}

	if len(localSuffix) > 0 {
		updated = lib.AppendToModfile("\n\n// Replace Local Deps\n\n" + localSuffix)

		lib.File.RunCmd("rm", "go.sum")
		lib.ModTidy()
	}
	return
}

// AppendToModfile appends provided string to end of mod file
func (lib *Library) AppendToModfile(text string) bool {
	// Open absolute path to mod file in append mode
	f, err := os.OpenFile(path.Join(lib.File.AbsPath(), "go.mod"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		lib.File.Output("Unable to open mod file: " + path.Join(lib.File.AbsPath(), "go.mod"))
		return false
	}

	// Append message
	if _, err := f.WriteString(text + "\n"); err != nil {
		lib.File.Output("Unable to write to mod file")
		return false
	}
	f.Close()

	lib.File.Debug("Appended " + text + " to mod file")

	// Write successful
	return true
}

// ModDeploy will commit and push local changes to the current branch before switching to master
func (lib *Library) ModDeploy(tag, commitMessage string) (deployed bool) {
	// Handle saving local changes
	lib.File.StashPop()
	lib.File.Add(".")

	// Ignore changes to go mod files (prevents committing local replacements)
	lib.File.Reset("go.*")

	message := ""
	if len(tag) == 0 {
		version := lib.File.Version

		if len(version) == 0 {
			message = "gomu: Deploy local changes"
		} else {
			message = "gomu: Deploy local changes before incrementing version from " + version
		}
	} else {
		message = "gomu: Deploy local changes before updating version to " + tag
	}

	if len(commitMessage) > 0 {
		message = commitMessage + "\n\n" + message
	}

	if lib.File.Commit(message) == nil {
		// Successful commit, push changes
		deployed = true
		lib.File.Output("Waiting to deploy local changes on sync...")
	} else {
		lib.File.Output("No changes to deploy!")
	}

	// Re-stash local changes to go.mod
	lib.File.Stash()

	return
}

// ModUpdate will refresh the current dir to master, reset mod files and push changes if there are any
func (lib *Library) ModUpdate(branch, commitMessage string) (err error) {
	lib.File.Output("Checking deps...")
	// Remove go.mod, ignore lib if not found (not a mod tracked lib)
	if lib.File.RunCmd("rm", "go.mod") != nil {
		lib.File.Output("No mod file found. Skipping.")
		return
	}

	// Reset mod files, or initialize if needed
	lib.File.RunCmd("git", "checkout", "go.mod")
	lib.ModInit()

	// Remove go sum to prevent mess from adding up
	if lib.File.RunCmd("rm", "go.sum") != nil {
		// No dependencies found. If this is unexpected for a given lib, something is out of sync
		lib.File.Output("No sum file found. No dependencies sorted.")
	}

	// Set versions from previous libs in chain
	lib.ModSetDeps()

	if err = lib.ModTidy(); err != nil {
		lib.File.Output("Mod tidy failed :(")
		return
	}

	if err = lib.File.Add("go.*"); err != nil {
		lib.File.Output("Git add failed :(")
		return
	}

	if err = lib.File.Commit(commitMessage); err == nil {
		lib.File.Output("Updating mod files...")
	} else {
		lib.File.Output("Deps up to date!")
	}

	if pushErr := lib.File.Push(); pushErr != nil {
		lib.File.Output("Push failed :( check local changes and commit status")
		return pushErr
	}

	lib.File.Output("Mod Sync Complete!")
	return
}
