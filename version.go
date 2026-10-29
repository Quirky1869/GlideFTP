package main

// appVersion is the running GlideFTP version, without the leading "v".
// make.sh overrides it at build time (-ldflags "-X main.appVersion=<ver>"), so
// a release binary always carries its tag even if this default was not bumped.
// Bump it with each release anyway: `wails dev` and manual builds use it as is.
var appVersion = "1.8.0"

// GetAppVersion returns the running version (shown in the Settings footer and
// compared with the latest GitHub release by CheckForUpdate).
func (a *App) GetAppVersion() string {
	return appVersion
}
