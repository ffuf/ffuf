package ffuf

import (
	"github.com/adrg/xdg"
	"path/filepath"
)

var (
	//VERSION holds the current version number
	VERSION = "2.1.1"
	//VERSION_APPENDIX holds additional version definition
	VERSION_APPENDIX = "-dev"
	CONFIGDIR        = filepath.Join(xdg.ConfigHome, "ffuf")
	HISTORYDIR       = filepath.Join(CONFIGDIR, "history")
	SCRAPERDIR       = filepath.Join(CONFIGDIR, "scraper")
	AUTOCALIBDIR     = filepath.Join(CONFIGDIR, "autocalibration")
)
