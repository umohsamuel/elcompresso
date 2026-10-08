package api

import "testing"

func TestMatchOrigin(t *testing.T) {
	cases := []struct {
		pattern, origin string
		want            bool
	}{
		{"https://elcompresso.vercel.app", "https://elcompresso.vercel.app", true},
		{"https://elcompresso.vercel.app", "https://evil.vercel.app", false},
		{"https://*.vercel.app", "https://elcompresso-git-main-me.vercel.app", true},
		{"https://*.vercel.app", "https://vercel.app", false},
		{"https://*.vercel.app", "https://evil.com/.vercel.app", false},
		{"https://*.vercel.app", "https://evil.com:443.vercel.app", false},
		{"https://*.vercel.app", "http://x.vercel.app", false},
	}
	for _, tc := range cases {
		if got := matchOrigin(tc.pattern, tc.origin); got != tc.want {
			t.Errorf("matchOrigin(%q, %q) = %v, want %v", tc.pattern, tc.origin, got, tc.want)
		}
	}
}
