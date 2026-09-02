package domain

import "context"

// SBOMScanner is the contract every Software Bill of Materials
// scanner backend must satisfy. The review use case consumes this
// interface, never the concrete adapter, so swapping osv-scanner
// for cdxgen, trivy, or a custom backend is a matter of wiring a
// different implementation in the composition root.
//
// All methods accept a context so the caller can cancel in-flight
// scans when the cobra cmd-level deadline fires (osv-scanner's
// network round-trips can take a while on a cold cache).
type SBOMScanner interface {
	// Scan walks the lockfiles in repoPath, queries the
	// vulnerability database, and returns every CVE the project's
	// dependencies are exposed to.
	//
	// An empty VulnerabilityResult (with no error) means "scan
	// completed, no vulnerabilities found". A nil repoPath returns
	// ErrNoRepoPath.
	//
	// The contract deliberately returns domain.Vulnerability
	// (flat, scanner-agnostic) instead of leaking osvschema types
	// into the use case.
	Scan(ctx context.Context, repoPath string) (VulnerabilityResult, error)
}