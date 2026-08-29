package depscan

import "testing"

func TestShouldScanDiffDetectsDependencyFiles(t *testing.T) {
	cases := []struct {
		name string
		diff string
		want bool
	}{
		{
			name: "package manifest changed",
			diff: "diff --git a/package.json b/package.json\n--- a/package.json\n+++ b/package.json\n",
			want: true,
		},
		{
			name: "lockfile changed",
			diff: "diff --git a/app/composer.lock b/app/composer.lock\n--- a/app/composer.lock\n+++ b/app/composer.lock\n",
			want: true,
		},
		{
			name: "dependency file deleted",
			diff: "diff --git a/go.sum b/go.sum\n--- a/go.sum\n+++ /dev/null\n",
			want: true,
		},
		{
			name: "source file changed",
			diff: "diff --git a/internal/app.go b/internal/app.go\n--- a/internal/app.go\n+++ b/internal/app.go\n",
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShouldScanDiff(tc.diff); got != tc.want {
				t.Fatalf("ShouldScanDiff() = %v, want %v", got, tc.want)
			}
		})
	}
}
