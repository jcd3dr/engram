package setup

import (
	"path/filepath"
	"strings"
	"testing"
)

// useIsolatedProfile keeps platform-resolved setup paths inside one disposable
// profile, including the Windows APPDATA path used by Gemini.
func useIsolatedProfile(t *testing.T) string {
	t.Helper()
	profile := t.TempDir()
	userHomeDir = func() (string, error) { return profile, nil }

	volume := filepath.VolumeName(profile)
	t.Setenv("APPDATA", filepath.Join(profile, "AppData", "Roaming"))
	t.Setenv("CODEX_HOME", "")
	t.Setenv("LOCALAPPDATA", filepath.Join(profile, "AppData", "Local"))
	t.Setenv("USERPROFILE", profile)
	t.Setenv("HOMEDRIVE", volume)
	t.Setenv("HOMEPATH", strings.TrimPrefix(profile, volume))
	t.Setenv("HOME", profile)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(profile, ".config"))

	return profile
}
