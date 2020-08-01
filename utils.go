package gomu

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/gomuserver/mod-utils/com"
	"github.com/gomuserver/mod-utils/sort"
	"github.com/remeh/sizedwaitgroup"
)

// exitWithErrorMessage prints message and exits
func exitWithErrorMessage(message string) {
	com.Println(message)
	exit(1)
}

// exit shows help then exits with status prints message and exits
func exit(status int) {
	os.Exit(status)
}

// showWarningOrQuit will exit if user declines warning
func showWarningOrQuit(message string) {
	if !ShowWarning(message) {
		com.Println("Exiting...")
		exit(0)
	}
}

// ShowWarning prints warning message and waits for user to confirm
func ShowWarning(message string) (ok bool) {
	if com.GetLogLevel() <= com.SILENT {
		// Don't show warnings for silent or name-only
		return true
	}

	var err error
	var text string
	reader := bufio.NewReader(os.Stdin)

	for err == nil {
		if text = strings.TrimSpace(text); len(text) > 0 {
			switch text {
			case "y", "Y", "Yes", "yes", "YES", "ok", "OK", "Ok":
				ok = true
				return
			default:
				com.Println("Nevermind then! :)")
				return
			}
		}

		// No newline. name-only already exited above
		fmt.Print(message + " [y|yes|ok]: ")
		text, err = reader.ReadString('\n')
	}

	com.Println("Oops... Something went wrong.")
	return
}

// Then handles cleanup after func
func cleanupStash(libs sort.StringArray) {
	closed = true

	waiter := sizedwaitgroup.New(runtime.GOMAXPROCS(0))

	// Resume working directory
	var f com.FileWrapper
	for i := range libs {
		f.Path = libs[i]

		waiter.Add()
		go func(f com.FileWrapper) {
			if f.StashPop() {
				f.Output("Warning - Has local changes")
			}

			waiter.Done()
		}(f)
	}

	waiter.Wait()
}

func (mu *MU) sync(lib Library, commitTitle, commitMessage string) {
	// Update the dep if necessary
	if err := lib.ModUpdate(mu.Options.Branch, commitTitle+"\n"+commitMessage); err == nil {
		// Dep was updated
		lib.File.Updated = true
		mu.Stats.UpdateCount++
		mu.Stats.UpdatedOutput += strconv.Itoa(mu.Stats.UpdateCount) + ") " + lib.File.GetGoURL() + "\n"
	}
}

func (mu *MU) pullRequest(lib Library, branch, commitTitle, commitMessage string) (err error) {
	if mu.Options.PullRequest {
		if len(branch) == 0 {
			branch, err = lib.File.CurrentBranch()
			if err != nil {
				return
			}
		}

		lib.File.Output("Attempting Pull Request " + branch + " to master...")

		resp, err := lib.File.PullRequest(commitTitle, commitMessage, branch, "master")
		if err == nil {
			mu.Stats.PRCount++
			mu.Stats.PROutput += resp.URL + "\n"
			lib.File.PROpened = true
			lib.File.Output("PR Created!")
		} else {
			if resp == nil || len(resp.Errors) == 0 {
				lib.File.Output("Failed to create PR :( " + err.Error())

			} else if strings.HasPrefix(resp.Errors[0].Message, "No commits between master and") {
				// No PR to create
			} else if strings.HasPrefix(resp.Errors[0].Message, "A pull request already exists for") {
				// PR Exists
			} else {
				lib.File.Output("Failed to create PR :(")
			}
		}
	}

	return
}

func (mu *MU) tag(lib Library) {
	if !mu.Options.Tag {
		// Ignore tagging entirely
		return
	}

	if lib.File.Version != "" {
		// Tag already set
		return
	}

	// Tag if forced or if able to increment
	if mu.Options.Tag && (len(mu.Options.SetVersion) > 0 || lib.ShouldTag()) {
		newTag := lib.TagLib(mu.Options.SetVersion)

		if len(newTag) > 0 {
			lib.File.Version = newTag
			lib.File.Tagged = true
			mu.Stats.TagCount++
			mu.Stats.TaggedOutput += strconv.Itoa(mu.Stats.TagCount) + ") " + lib.File.GetGoURL() + " " + lib.File.Version + "\n"
		}
	}

	// Set tag for next lib if not set
	if len(lib.File.Version) == 0 {
		lib.File.Version = lib.GetLatestTag()
	}
}

func (mu *MU) removeBranchIfUnused(lib Library) {
	if !lib.File.BranchCreated {
		// Don't delete branches that were not created this session
		return
	}

	// Check if created a branch we didn't need
	if !lib.File.Updated && !lib.File.Committed && !lib.File.PROpened {
		switch mu.Options.Branch {
		case "master", "develop", "staging", "beta", "prod", "":
			// Ignore protected branches and empty branch
		default:
			// Delete branch
			lib.File.CheckoutBranch("master")
			if lib.File.RunCmd("git", "branch", "-D", mu.Options.Branch) == nil {
				// No longer needed
				lib.File.BranchCreated = false

				lib.File.RunCmd("git", "push", "origin", "--delete", mu.Options.Branch)
				if !closed {
					lib.File.Output("Newly created branch did not update. Deleted unused branch")
				}
			}
		}
	} else {
		mu.Stats.CreatedCount++
		mu.Stats.CreatedOutput += strconv.Itoa(mu.Stats.CreatedCount) + ") " + lib.File.Path + "#" + mu.Options.Branch + "\n"
	}
}

func (mu *MU) getCommitDetails(lib Library) (commitTitle, commitMessage string) {
	commitTitle = mu.Options.CommitMessage
	if len(commitTitle) == 0 {
		commitTitle = "Update Mod Files"
	}

	commitTitle = "gomu: " + commitTitle
	commitMessage = ""
	for itr := lib.updatedDeps; itr != nil; itr = itr.Next {
		url := itr.File.GetGoURL()

		if itr.File.Updated {
			commitMessage += "\nUpdated " + url + "@" + itr.File.Version
		} else {
			commitMessage += "\nSet " + url + "@" + itr.File.Version
		}
	}

	return
}

func (mu *MU) commit(lib Library) {
	if mu.Options.Commit {
		lib.File.Output("Checking for local changes...")
		lib.File.Committed = lib.ModDeploy("", mu.Options.CommitMessage)

		if lib.File.Committed {
			mu.Stats.CommitCount++
			mu.Stats.DeployedOutput += strconv.Itoa(mu.Stats.CommitCount) + ") " + lib.File.GetGoURL() + "\n"
		}
	}
}

func (mu *MU) replace(lib Library, fileHead *sort.FileNode) {
	lib.File.Output("Checking deps...")

	// Aggregate updated versions of previously parsed deps
	lib.ModAddDeps(fileHead, true)

	if lib.updatedDeps == nil {
		lib.File.Output("Skipping: No deps in chain to set.")
	} else {
		lib.File.Output("Setting local replacements...")

		// Append local replacements for all libs in lib.updatedDeps
		if lib.ModReplaceLocal() {
			lib.File.Updated = true
			mu.Stats.UpdateCount++
			mu.Stats.UpdatedOutput += strconv.Itoa(mu.Stats.UpdateCount) + ") " + lib.File.Path + "\n"

			lib.File.Output("Local replacements set!")
		} else {
			lib.File.Output("Failed to set local deps :(")
		}
	}
}

func (mu *MU) addSecret(lib Library) (err error) {
	// Get secret name from filepath
	_, secretName := path.Split(mu.Options.SourcePath)
	secretFile, err := os.Open(mu.Options.SourcePath)
	if err != nil {
		return err
	}
	defer secretFile.Close()

	// Read secret from file
	var body []byte
	if body, err = ioutil.ReadAll(secretFile); err != nil {
		return
	}

	// Set secret with filename on repo
	if err = lib.File.AddSecret(secretName, string(body)); err != nil {
		lib.File.Output("Unable to add secret :(")
		return
	}

	return
}

func (mu *MU) test(lib Library, fileHead *sort.FileNode) (err error) {
	if lib.File.StashPop() {
		// Local changes exist
		lib.File.Output("Applying local changes...")
	}

	// Only set updated deps
	lib.ModAddDeps(fileHead, false)

	if lib.updatedDeps != nil {
		lib.File.Output("Setting dep versions...")
		lib.ModSetDeps()
	}

	lib.File.Output("Building...")
	// Try building
	if err = lib.File.RunCmd("go", "build", "-o", "test-out.o"); err != nil {
		err = nil
		// Try plugin mode
		if err = lib.File.RunCmd("go", "build", "-buildmode=plugin", "-o", "test-out.o"); err != nil {
			lib.File.Output("Build failed :(")
			lib.File.TestFailed = true
			mu.Stats.TestFailedCount++
			mu.Stats.TestFailedOutput += strconv.Itoa(mu.Stats.TestFailedCount) + ") " + lib.File.Path

			mu.Stats.TestFailedOutput += "\n"
			return
		}
	}

	lib.File.Output("Build Succeeded!")
	lib.File.RunCmd("rm", "test-out.o")

	lib.File.Output("Testing...")
	output, err := lib.File.CmdOutput("go", "test")

	if err == nil {
		if strings.Contains(output, "PASS") {
			lib.File.Output("Test Passed!")
		} else {
			lib.File.Output("No tests to run.")
		}

	} else {
		lib.File.Output("Test failed :(")

		// Tag failures as updated for stats
		lib.File.TestFailed = true
		mu.Stats.TestFailedCount++
		mu.Stats.TestFailedOutput += strconv.Itoa(mu.Stats.TestFailedCount) + ") " + lib.File.Path

		mu.Stats.TestFailedOutput += "\n"
	}

	return
}

func (mu *MU) reset(lib Library) {
	if len(mu.Options.Branch) > 0 {
		lib.File.Output("Reverting mod files to <" + mu.Options.Branch + "> ref...")
	} else {
		lib.File.Output("Reverting mod files to last-committed ref...")
	}

	lib.File.StashPop()

	// Revert any changes to mod files
	lib.File.RunCmd("git", "checkout", mu.Options.Branch, "go.mod")
	lib.File.RunCmd("git", "checkout", mu.Options.Branch, "go.sum")

	lib.File.Output("Reverted mod files!")

	if lib.File.HasChanges() {
		lib.File.Output("Warning! Has local changes.")
	}
}

func (mu *MU) pull(lib Library) {
	// Check out branch if provided
	if len(mu.Options.Branch) > 0 {
		lib.File.Output("Checking out " + mu.Options.Branch + "...")

		if lib.File.CheckoutBranch(mu.Options.Branch) != nil {
			lib.File.Output("Failed to check out branch :(")
		}
	}

	lib.File.Output("Pulling latest changes...")

	if lib.File.Pull() == nil {
		lib.File.Output("Updated successfully!")

		lib.File.Updated = true
		mu.Stats.UpdateCount++
		mu.Stats.UpdatedOutput += strconv.Itoa(mu.Stats.UpdateCount) + ") " + lib.File.Path

		mu.Stats.UpdatedOutput += "\n"
	} else {
		lib.File.Output("Failed to update :(")
	}
}

func (mu *MU) updateOrCreateBranch(lib Library) (switched, created bool, err error) {
	lib.File.Output("Updating refs...")
	if len(lib.File.Version) == 0 && !mu.Options.Tag {
		// TODO: Improve the performance of this check by explicitly looking at commit tag?
		oldTag := lib.GetLatestTag()
		if len(oldTag) > 0 {
			// Check if updated
			lib.File.Fetch()
			newTag := lib.GetLatestTag()

			if oldTag != newTag {
				// Force version update
				lib.File.Output("Tag was out of date, setting explicit version.")
				lib.File.Version = newTag
				lib.File.Tagged = true
			}
		}
	} else {
		// Version already set, just update
		lib.File.Fetch()
	}

	if len(mu.Options.Branch) > 0 {
		switched, created, err = lib.File.CheckoutOrCreateBranch(mu.Options.Branch)
		if err != nil {
			lib.File.Error("Failed to checkout " + mu.Options.Branch + " :(")
			return
		} else if !switched {
			lib.File.Output("Already on " + mu.Options.Branch)
		} else if !created {
			lib.File.Output("Switched to " + mu.Options.Branch)
		} else {
			lib.File.Output("Created branch " + mu.Options.Branch + "!")
			lib.File.RunCmd("git", "push", "-u", "origin", mu.Options.Branch)

			if mu.Options.Action == "pull" {
				// This won't be deleted
				mu.Stats.CreatedCount++
				mu.Stats.CreatedOutput += strconv.Itoa(mu.Stats.CreatedCount) + ") " + lib.File.Path + "#" + mu.Options.Branch + "\n"
			}
		}
	}

	lib.File.Output("Pulling latest changes...")

	if err = lib.File.Pull(); err != nil {
		lib.File.Output("Failed to pull " + mu.Options.Branch + " :(")
	}

	return
}
