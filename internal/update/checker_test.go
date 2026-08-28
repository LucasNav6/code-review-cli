package update

import "testing"

func TestCheckWith_NoRepoConfigured(t *testing.T) {
	result, err := checkWith("", "v1.0.0", failFetch(t), noCache, noSave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != nil {
		t.Fatalf("expected nil result when no repo is configured, got %+v", result)
	}
}

func TestCheckWith_DevBuildSkipsCheck(t *testing.T) {
	result, err := checkWith("owner/repo", "dev", failFetch(t), noCache, noSave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != nil {
		t.Fatalf("expected nil result for a dev build, got %+v", result)
	}
}

func TestCheckWith_UsesCacheWhenFresh(t *testing.T) {
	cache := &cacheEntry{LatestVersion: "v1.2.0"}

	result, err := checkWith(
		"owner/repo",
		"v1.0.0",
		failFetch(t),
		func() (*cacheEntry, bool) { return cache, true },
		noSave,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil || !result.HasUpdate || result.Latest != "v1.2.0" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCheckWith_NoUpdateWhenCurrent(t *testing.T) {
	saved := ""

	result, err := checkWith(
		"owner/repo",
		"v1.2.0",
		func(repo string) (*Release, error) { return &Release{TagName: "v1.2.0"}, nil },
		noCache,
		func(v string) { saved = v },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil || result.HasUpdate {
		t.Fatalf("expected no update available, got %+v", result)
	}

	if saved != "v1.2.0" {
		t.Fatalf("expected fetched version to be cached, got %q", saved)
	}
}

func TestCheckWith_HasUpdateWhenNewer(t *testing.T) {
	result, err := checkWith(
		"owner/repo",
		"v1.0.0",
		func(repo string) (*Release, error) { return &Release{TagName: "v2.0.0"}, nil },
		noCache,
		noSave,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil || !result.HasUpdate {
		t.Fatalf("expected an update to be available, got %+v", result)
	}

	if result.ReleaseURL != "https://github.com/owner/repo/releases/tag/v2.0.0" {
		t.Fatalf("unexpected release url: %q", result.ReleaseURL)
	}
}

func failFetch(t *testing.T) latestReleaseFunc {
	return func(repo string) (*Release, error) {
		t.Fatalf("fetchLatest should not be called for repo %q", repo)
		return nil, nil
	}
}

func noCache() (*cacheEntry, bool) { return nil, false }
func noSave(string)                {}
