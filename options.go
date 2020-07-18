package gomu

import (
	"strings"

	"github.com/gomuserver/mod-utils/com"
	"github.com/gomuserver/mod-utils/sort"
)

// Options represents different settings to perform an action
type Options struct {
	Action string `json:"action,-"` // Not supported from server

	Branch        string `json:"branch"`
	CommitMessage string `json:"message"`

	Commit      bool   `json:"commit,-"` // Not supported from server
	PullRequest bool   `json:"createPR"`
	Tag         bool   `json:"shouldTag"`
	SetVersion  string `json:"setVersion"`

	SourcePath string `json:"source,-"` // Not supported from server

	DirectImport       bool             `json:"direct"`
	TargetDirectories  sort.StringArray `json:"searchLibs"` // Not supported from server
	FilterDependencies sort.StringArray `json:"syncLibs"`

	LogLevel com.LogLevel
}

// New returns new Mod Utils struct
func New(options Options) *MU {
	var mu MU
	mu.Stats.Options = &options
	mu.Options = options
	return &mu
}

// Format will wrap options data into a printable output string
func (o *Options) Format() (output string) {
	warningActions := []string{"Sync action will:"}
	if o.Branch != "" {
		warningActions = append(warningActions, "- checkout (or create) branch "+o.Branch)
	}
	warningActions = append(warningActions, "- update mod files")
	if o.Commit {
		warningActions = append(warningActions, "- commit local changes (if any)")
	}
	if o.PullRequest {
		warningActions = append(warningActions, "- open pull request for changes (if any)")
	}
	if o.Tag {
		if len(o.SetVersion) > 0 {
			warningActions = append(warningActions, "- tag all dependencies "+o.SetVersion)
		} else {
			warningActions = append(warningActions, "- increment tag version (if updated)")
		}
	}

	msg := strings.Join(warningActions, "\n  ")
	msg += "\n\nOn repositories: " + o.FilterDependencies.String()
	msg += "\nIn directories: " + o.TargetDirectories.String()

	return msg
}
