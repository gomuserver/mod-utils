package gomu

import (
	"github.com/gomuserver/mod-utils/com"
	"github.com/gomuserver/mod-utils/sort"
)

// Library represents a file and its updated deps
type Library struct {
	File *com.FileWrapper

	updatedDeps *sort.FileNode
}

// LibraryFromPath returns a library reference for a filepath
func LibraryFromPath(filepath string) *Library {
	return &Library{File: &com.FileWrapper{Path: filepath}}
}

// AddDep will ensure go.mod sets specific version of node.file when syncing
func (lib *Library) AddDep(node *sort.FileNode) {
	node.InsertInto(&lib.updatedDeps)
}

func performPull(branch string, itr *sort.FileNode) (success bool) {
	success = true

	if itr.File.CheckoutBranch(branch) != nil {
		itr.File.Output("Failed to checkout " + branch + " :(")
		success = false
	}

	if itr.File.Pull() == nil {
		itr.File.Output("Pull successful!")
	} else {
		itr.File.Output("Failed to pull " + branch + " :(")
		success = false
	}

	return
}
