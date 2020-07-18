package sort

import (
	"os"
	"path"
	"strings"

	"github.com/gomuserver/mod-utils/com"
)

// SortedRecursiveDeps returns a linked list of FileNodes directly or indirectly depending on provided filters
// Note returns all libs if no filters provided
func (libs StringArray) SortedRecursiveDeps(subDeps StringArray) (listHead *FileNode, count int) {
	// Parse filters
	filters := make([]*com.FileWrapper, len(subDeps))
	for i := range subDeps {
		var f com.FileWrapper
		filterComps := strings.Split(subDeps[i], "@")
		if len(filterComps) > 1 {
			f.Path = filterComps[0]
			f.Version = filterComps[1]
		} else {
			f.Path = subDeps[i]
		}
		filters[i] = &f
	}

	// Parse each lib and add if included by a filter or if no filters provided
	for i := range libs {
		var node FileNode
		var file com.FileWrapper
		node.File = &file
		node.File.Path = strings.TrimSpace(libs[i])

		if len(node.File.Path) == 0 {
			// Ignore if no file name
			continue
		}

		f, err := os.Open(path.Join(node.File.Path, ".git"))
		if err != nil {
			// Ignore if not a repo
			continue
		}
		defer f.Close()

		// Add file to list if no filters are provided, or if file depends on any of the filter deps
		if len(filters) == 0 || node.File.MatchesAny(filters) || node.File.DependsOnAny(filters) {
			// Insert file
			node.InsertInto(&listHead)
			count++
		}
	}

	return
}

// SortedDirectDeps returns a linked list of FileNodes depending on provided filters
// Note returns all libs if no filters provided
func (libs StringArray) SortedDirectDeps(subDeps StringArray) (listHead *FileNode, count int) {
	// Parse filters
	filters := make([]*com.FileWrapper, len(subDeps))
	for i := range subDeps {
		var f com.FileWrapper
		filterComps := strings.Split(subDeps[i], "@")
		if len(filterComps) > 1 {
			f.Path = filterComps[0]
			f.Version = filterComps[1]
		} else {
			f.Path = subDeps[i]
		}
		filters[i] = &f
	}

	// Parse each lib and add if included by a filter or if no filters provided
	for i := range libs {
		var node FileNode
		var file com.FileWrapper
		node.File = &file
		node.File.Path = strings.TrimSpace(libs[i])

		if len(node.File.Path) == 0 {
			// Ignore if no file name
			continue
		}

		f, err := os.Open(path.Join(node.File.Path, ".git"))
		if err != nil {
			// Ignore if not a repo
			continue
		}
		defer f.Close()

		// Add file to list if no filters are provided, or if file depends on any of the filter deps
		if len(filters) == 0 || node.File.MatchesAny(filters) || node.File.DirectlyImportsAny(filters) {
			// Insert file
			node.InsertInto(&listHead)
			count++
		}
	}

	return
}
