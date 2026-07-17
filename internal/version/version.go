package version

// Version is set at link time by GoReleaser / build scripts.
// Default "dev" for local builds.
var Version = "dev"

// String returns a display form (always with leading v when semver-like).
func String() string {
	if Version == "" || Version == "dev" {
		return "dev"
	}
	if Version[0] == 'v' || Version[0] == 'V' {
		return Version
	}
	return "v" + Version
}
