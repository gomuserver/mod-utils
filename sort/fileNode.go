package sort

import "github.com/gomuserver/mod-utils/com"

// FileNode represents a file path within a linked list of sorted dependencies
type FileNode struct {
	File *com.FileWrapper

	// Next file in sorted chain
	Last *FileNode
	// Last file in sorted chain
	Next *FileNode
}
