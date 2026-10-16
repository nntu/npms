package version

import (
	"runtime/debug"
	"strings"
)

var (
	// Version holds the current semantic version of NPMS.
	// Can be overridden at build time via -ldflags.
	Version = "1.0.0"

	// GitCommit holds the git commit hash at build time.
	// Can be overridden at build time via -ldflags.
	GitCommit = ""

	// BuildDate holds the UTC ISO-8601 build timestamp.
	// Can be overridden at build time via -ldflags.
	BuildDate = ""
)

// Info contains build and version details.
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit,omitempty"`
	BuildDate string `json:"build_date,omitempty"`
}

// Get returns the populated build information, falling back to Go module build info if not injected via ldflags.
func Get() Info {
	info := Info{
		Version:   strings.TrimSpace(Version),
		GitCommit: strings.TrimSpace(GitCommit),
		BuildDate: strings.TrimSpace(BuildDate),
	}
	if info.Version == "" {
		info.Version = "1.0.0"
	}
	if info.GitCommit == "" || info.BuildDate == "" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range bi.Settings {
				if setting.Key == "vcs.revision" && info.GitCommit == "" {
					rev := setting.Value
					if len(rev) > 7 {
						rev = rev[:7]
					}
					info.GitCommit = rev
				}
				if setting.Key == "vcs.time" && info.BuildDate == "" {
					info.BuildDate = setting.Value
				}
			}
		}
	}
	return info
}
