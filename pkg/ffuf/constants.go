package ffuf

import (
	"github.com/adrg/xdg"
	"path/filepath"
)

var (
	// VERSION is the linker injection target for the release version: goreleaser
	// sets it to the git tag via -ldflags -X (see .goreleaser.yml). Do not delete
	// it because nothing in this package appears to read it; the linker reference
	// is invisible to a Go-source search, and -X against a missing symbol fails
	// silently. The literal below is a placeholder, not a real version, and is
	// only ever seen when the ldflags injection, the VCS metadata and the module
	// version are all unavailable.
	VERSION = "0.0.0"
	// VERSION_APPENDIX marks a non-release build. goreleaser empties it via -X,
	// which is how Version() tells a release binary from a development one.
	VERSION_APPENDIX = "-dev"
	CONFIGDIR        = filepath.Join(xdg.ConfigHome, "ffuf")
	HISTORYDIR       = filepath.Join(CONFIGDIR, "history")
	SCRAPERDIR       = filepath.Join(CONFIGDIR, "scraper")
	AUTOCALIBDIR     = filepath.Join(CONFIGDIR, "autocalibration")
)
