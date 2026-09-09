package ffuf

import (
	"strings"
	"testing"
)

// selectVersion holds the whole decision, so every build path can be driven
// deterministically without needing a binary built that way.
func TestSelectVersion(t *testing.T) {
	cases := []struct {
		name         string
		injected     string
		appendix     string
		git          string
		module       string
		wantVersion  string
		wantReleased bool
	}{
		{
			name:     "release build reports the injected tag",
			injected: "2.3.0", appendix: "", git: "git-20260909-256c1b02", module: "2.3.0",
			wantVersion: "2.3.0", wantReleased: true,
		},
		{
			name:     "source checkout reports the git identifier",
			injected: "0.0.0", appendix: "-dev", git: "git-20260909-256c1b02", module: "",
			wantVersion: "git-20260909-256c1b02", wantReleased: false,
		},
		{
			name:     "vcs metadata outranks the module version so a dirty tree cannot claim a release",
			injected: "0.0.0", appendix: "-dev", git: "git-20260909-256c1b02-dirty", module: "2.3.0",
			wantVersion: "git-20260909-256c1b02-dirty", wantReleased: false,
		},
		{
			name:     "go install reports the module version it was built from",
			injected: "0.0.0", appendix: "-dev", git: "", module: "2.2.1",
			wantVersion: "2.2.1", wantReleased: true,
		},
		{
			name:     "no metadata at all falls back to the placeholder",
			injected: "0.0.0", appendix: "-dev", git: "", module: "",
			wantVersion: "0.0.0-dev", wantReleased: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, released := selectVersion(tc.injected, tc.appendix, tc.git, tc.module)
			if got != tc.wantVersion {
				t.Errorf("version = %q, want %q", got, tc.wantVersion)
			}
			if released != tc.wantReleased {
				t.Errorf("released = %v, want %v", released, tc.wantReleased)
			}
		})
	}
}

// A git identifier must never be reported as a released version, because
// FormattedVersion would then render it as "vgit-20260909-...".
func TestSelectVersion_GitIdentifierIsNeverReleased(t *testing.T) {
	for _, module := range []string{"", "2.3.0", "2.3.1-0.20260909191139-256c1b024995"} {
		got, released := selectVersion("0.0.0", "-dev", "git-20260909-256c1b02", module)
		if released {
			t.Errorf("git identifier %q reported as released with module=%q", got, module)
		}
	}
}

// normalizeModuleVersion decides whether the module version recorded by
// `go install module@version` is usable. The toolchain writes "(devel)" when
// there is no version, and prefixes real ones with "v".
func TestNormalizeModuleVersion(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"exact release tag", "v2.3.0", "2.3.0"},
		{"prerelease tag", "v2.3.0-rc1", "2.3.0-rc1"},
		{"pseudo version for an untagged commit", "v2.3.1-0.20260909191139-256c1b024995", "2.3.1-0.20260909191139-256c1b024995"},
		{"local build placeholder", "(devel)", ""},
		{"no version recorded", "", ""},
		{"already unprefixed", "2.3.0", "2.3.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeModuleVersion(tc.in); got != tc.want {
				t.Errorf("normalizeModuleVersion(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// FormattedVersion adds a "v" only for released versions, so it must never
// produce a v-prefixed git identifier whatever this test binary was built from.
func TestFormattedVersion_MatchesVersion(t *testing.T) {
	v := Version()
	formatted := FormattedVersion()
	if formatted != v && formatted != "v"+v {
		t.Errorf("FormattedVersion() = %q, want %q or %q", formatted, v, "v"+v)
	}
	if strings.HasPrefix(formatted, "vgit-") {
		t.Errorf("FormattedVersion() = %q, a git identifier must not be v-prefixed", formatted)
	}
}

// Version() fills in the default User-Agent on every request, so the build
// metadata lookups behind it are resolved once rather than per call.
func TestVersion_IsResolvedOnce(t *testing.T) {
	first, firstReleased := resolveVersion()
	second, secondReleased := resolveVersion()
	if first != second || firstReleased != secondReleased {
		t.Errorf("resolveVersion() is not stable: (%q,%v) then (%q,%v)", first, firstReleased, second, secondReleased)
	}
}

// The constant is a linker injection target, not a version to maintain by hand.
// It sat at "2.1.0" through the 2.2.0, 2.2.1 and 2.3.0 releases, which is what a
// binary from `go install module@version` reported.
func TestVersion_ConstantIsAPlaceholder(t *testing.T) {
	if VERSION != "0.0.0" {
		t.Errorf("VERSION = %q, want the 0.0.0 placeholder; a real version here goes stale silently", VERSION)
	}
	if VERSION_APPENDIX == "" {
		t.Error("VERSION_APPENDIX must be non-empty in source, it is what marks a build as not-a-release")
	}
}

func BenchmarkVersion(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Version()
	}
}
