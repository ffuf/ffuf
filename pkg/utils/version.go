package utils

import (
	"fmt"
)

var (
	//VERSION holds the current version number
	VERSION = "1.5.0"
	//VERSION_APPENDIX holds additional version definition
	VERSION_APPENDIX = "-dev"
)

// Version returns the ffuf version string
func Version() string {
	return fmt.Sprintf("%s%s", VERSION, VERSION_APPENDIX)
}
