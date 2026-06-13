package semver

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input      string
		wantMajor  int
		wantMinor  int
		wantPatch  int
		wantPre    string
		wantErr    bool
	}{
		{input: "1.2.3", wantMajor: 1, wantMinor: 2, wantPatch: 3},
		{input: "^1.2.3", wantMajor: 1, wantMinor: 2, wantPatch: 3},
		{input: "~1.2.3", wantMajor: 1, wantMinor: 2, wantPatch: 3},
		{input: ">=1.0.0", wantMajor: 1, wantMinor: 0, wantPatch: 0},
		{input: "v2.0.0", wantMajor: 2, wantMinor: 0, wantPatch: 0},
		{input: "1.2.3-beta.1", wantMajor: 1, wantMinor: 2, wantPatch: 3, wantPre: "beta.1"},
		{input: "0.0.1", wantMajor: 0, wantMinor: 0, wantPatch: 1},
		{input: "10.20.30", wantMajor: 10, wantMinor: 20, wantPatch: 30},
		{input: "1.0", wantMajor: 1, wantMinor: 0, wantPatch: 0},
		{input: "1", wantMajor: 1, wantMinor: 0, wantPatch: 0},
		{input: "1.2.3-alpha", wantMajor: 1, wantMinor: 2, wantPatch: 3, wantPre: "alpha"},
		{input: "", wantErr: true},
		{input: "abc", wantErr: true},
		{input: "1.2.x", wantErr: true},
		{input: "^", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := Parse(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("Parse(%q) expected error, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Errorf("Parse(%q) unexpected error: %v", tc.input, err)
				return
			}
			if got.Major != tc.wantMajor {
				t.Errorf("Parse(%q).Major = %d, want %d", tc.input, got.Major, tc.wantMajor)
			}
			if got.Minor != tc.wantMinor {
				t.Errorf("Parse(%q).Minor = %d, want %d", tc.input, got.Minor, tc.wantMinor)
			}
			if got.Patch != tc.wantPatch {
				t.Errorf("Parse(%q).Patch = %d, want %d", tc.input, got.Patch, tc.wantPatch)
			}
			if got.PreRelease != tc.wantPre {
				t.Errorf("Parse(%q).PreRelease = %q, want %q", tc.input, got.PreRelease, tc.wantPre)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	mustParse := func(s string) Version {
		v, err := Parse(s)
		if err != nil {
			t.Fatalf("mustParse(%q) failed: %v", s, err)
		}
		return v
	}

	tests := []struct {
		a    string
		b    string
		want int
	}{
		// equal versions
		{a: "1.2.3", b: "1.2.3", want: 0},
		{a: "0.0.0", b: "0.0.0", want: 0},
		// major diff
		{a: "2.0.0", b: "1.0.0", want: 1},
		{a: "1.0.0", b: "2.0.0", want: -1},
		// minor diff
		{a: "1.2.0", b: "1.1.0", want: 1},
		{a: "1.1.0", b: "1.2.0", want: -1},
		// patch diff
		{a: "1.0.2", b: "1.0.1", want: 1},
		{a: "1.0.1", b: "1.0.2", want: -1},
		// pre-release < release
		{a: "1.0.0-alpha", b: "1.0.0", want: -1},
		// release > pre-release
		{a: "1.0.0", b: "1.0.0-alpha", want: 1},
		// both pre-release: lexicographic
		{a: "1.0.0-alpha", b: "1.0.0-beta", want: -1},
		{a: "1.0.0-beta", b: "1.0.0-alpha", want: 1},
		{a: "1.0.0-alpha", b: "1.0.0-alpha", want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.a+"_vs_"+tc.b, func(t *testing.T) {
			got := Compare(mustParse(tc.a), mustParse(tc.b))
			if got != tc.want {
				t.Errorf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestClassifyUpdate(t *testing.T) {
	mustParse := func(s string) Version {
		v, err := Parse(s)
		if err != nil {
			t.Fatalf("mustParse(%q) failed: %v", s, err)
		}
		return v
	}

	tests := []struct {
		current string
		latest  string
		want    UpdateType
	}{
		{current: "1.0.0", latest: "1.0.1", want: Patch},
		{current: "1.0.0", latest: "1.1.0", want: Minor},
		{current: "1.0.0", latest: "2.0.0", want: Major},
		{current: "1.0.0", latest: "1.0.0", want: None},
		{current: "2.0.0", latest: "1.0.0", want: None},
		{current: "1.2.3", latest: "1.2.4", want: Patch},
		{current: "1.2.3", latest: "1.3.0", want: Minor},
		{current: "1.2.3", latest: "2.0.0", want: Major},
	}

	for _, tc := range tests {
		t.Run(tc.current+"->"+tc.latest, func(t *testing.T) {
			got := ClassifyUpdate(mustParse(tc.current), mustParse(tc.latest))
			if got != tc.want {
				t.Errorf("ClassifyUpdate(%q, %q) = %q, want %q", tc.current, tc.latest, got, tc.want)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		v    Version
		want string
	}{
		{v: Version{Major: 1, Minor: 2, Patch: 3}, want: "1.2.3"},
		{v: Version{Major: 0, Minor: 0, Patch: 0}, want: "0.0.0"},
		{v: Version{Major: 1, Minor: 2, Patch: 3, PreRelease: "alpha"}, want: "1.2.3-alpha"},
		{v: Version{Major: 1, Minor: 2, Patch: 3, PreRelease: "beta.1"}, want: "1.2.3-beta.1"},
		{v: Version{Major: 10, Minor: 20, Patch: 30}, want: "10.20.30"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := tc.v.String()
			if got != tc.want {
				t.Errorf("Version.String() = %q, want %q", got, tc.want)
			}
		})
	}
}
