package eooaoo

import (
	"runtime/debug"
	"sync"
)

const modulePath = "github.com/lsbjordao/go-iucn-eoo-aoo"

// BuildInfo identifies the compiled code. Fields are only filled from metadata
// the Go toolchain embedded; a local build without VCS stamping reports no
// revision rather than an invented one.
type BuildInfo struct {
	GoVersion     string `json:"go_version"`
	ModuleVersion string `json:"module_version,omitempty"`
	VCSRevision   string `json:"vcs_revision,omitempty"`
	VCSTime       string `json:"vcs_time,omitempty"`
	VCSModified   *bool  `json:"vcs_modified,omitempty"`
}

var readBuild = sync.OnceValue(func() *BuildInfo {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	out := &BuildInfo{GoVersion: bi.GoVersion}
	if bi.Main.Path == modulePath {
		if bi.Main.Version != "(devel)" {
			out.ModuleVersion = bi.Main.Version
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				out.VCSRevision = s.Value
			case "vcs.time":
				out.VCSTime = s.Value
			case "vcs.modified":
				v := s.Value == "true"
				out.VCSModified = &v
			}
		}
		return out
	}
	for _, d := range bi.Deps {
		if d.Path == modulePath {
			if d.Replace != nil {
				d = d.Replace
			}
			out.ModuleVersion = d.Version
		}
	}
	return out
})

// Build returns the embedded build metadata, or nil when unavailable.
func Build() *BuildInfo {
	b := readBuild()
	if b == nil {
		return nil
	}
	c := *b
	return &c
}
