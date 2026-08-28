package githubpr

import "testing"

func TestParseURL_Valid(t *testing.T) {
	pr, err := ParseURL("https://github.com/quadminds/route-cost/pull/482")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pr.Owner != "quadminds" || pr.Repo != "route-cost" || pr.Number != 482 {
		t.Fatalf("unexpected pr: %+v", pr)
	}

	if pr.Repository() != "quadminds/route-cost" {
		t.Fatalf("unexpected repository: %q", pr.Repository())
	}
}

func TestParseURL_Invalid(t *testing.T) {
	cases := []string{
		"https://gitlab.com/org/repo/pull/1",
		"https://github.com/org/repo",
		"https://github.com/org/repo/issues/1",
		"https://github.com/org/repo/pull/abc",
		"not a url at all",
	}

	for _, raw := range cases {
		if _, err := ParseURL(raw); err == nil {
			t.Errorf("ParseURL(%q) expected error, got nil", raw)
		}
	}
}

func TestNew(t *testing.T) {
	pr := New("org", "repo", 123)

	if pr.Repository() != "org/repo" {
		t.Fatalf("unexpected repository: %q", pr.Repository())
	}

	if pr.URL != "https://github.com/org/repo/pull/123" {
		t.Fatalf("unexpected url: %q", pr.URL)
	}
}
