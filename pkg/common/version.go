package common

// NAME of the App
var NAME = "file_exporter"

// SUMMARY of the Version
var SUMMARY = "1.0.0-dev"

// BRANCH of the Version
var BRANCH = "dev"

// VERSION of Release
var VERSION = "1.3.0"

// AppVersion --
var AppVersion AppVersionInfo

// AppVersionInfo --
type AppVersionInfo struct {
	Name    string
	Version string
	Branch  string
	Summary string
}

func init() {
	AppVersion = AppVersionInfo{
		Name:    NAME,
		Version: VERSION,
		Branch:  BRANCH,
		Summary: SUMMARY,
	}
}
