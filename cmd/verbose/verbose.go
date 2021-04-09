package verbose

import "log"

var enableDebugOutput = false

// EnableDebugOutput enables debugging output
func EnableDebugOutput() {
	enableDebugOutput = true
}

// Debugf prints log if debugging is enabled
func Debugf(format string, v ...interface{}) {
	if enableDebugOutput {
		log.Printf(format, v...)
	}
}
