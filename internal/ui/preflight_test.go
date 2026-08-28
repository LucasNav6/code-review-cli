package ui

import "testing"

func TestAllDependenciesFound(t *testing.T) {
	cases := []struct {
		name    string
		results []dependencyResult
		want    bool
	}{
		{"empty", nil, true},
		{"all found", []dependencyResult{{Found: true}, {Found: true}}, true},
		{"one missing", []dependencyResult{{Found: true}, {Found: false}}, false},
		{"all missing", []dependencyResult{{Found: false}, {Found: false}}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := allDependenciesFound(tc.results); got != tc.want {
				t.Fatalf("allDependenciesFound(%+v) = %v, want %v", tc.results, got, tc.want)
			}
		})
	}
}

func TestCheckDependenciesCmd_DetectsMissingBinary(t *testing.T) {
	original := requiredDependencies
	defer func() { requiredDependencies = original }()

	requiredDependencies = []dependency{
		{Label: "Go toolchain", Command: "go"},
		{Label: "definitely not a real command", Command: "code-review-definitely-missing-binary"},
	}

	msg := checkDependenciesCmd()().(preflightMsg)

	if len(msg.results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(msg.results))
	}

	if !msg.results[0].Found {
		t.Fatalf("expected %q to be found", requiredDependencies[0].Command)
	}

	if msg.results[1].Found {
		t.Fatalf("expected %q to be reported as missing", requiredDependencies[1].Command)
	}

	if allDependenciesFound(msg.results) {
		t.Fatal("expected allDependenciesFound to be false when one binary is missing")
	}
}
