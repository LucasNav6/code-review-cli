// Package githubpr modela un Pull Request de GitHub y sabe interpretarlo
// tanto a partir de una URL completa como de sus partes (org/repo/id).
package githubpr

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// PullRequest identifica un Pull Request puntual de GitHub.
type PullRequest struct {
	Owner  string
	Repo   string
	Number int
	URL    string
}

// Repository devuelve el identificador "owner/repo" tal como lo espera `gh`.
func (pr PullRequest) Repository() string {
	return pr.Owner + "/" + pr.Repo
}

// New construye un PullRequest a partir de sus componentes.
func New(owner, repo string, number int) *PullRequest {
	return &PullRequest{
		Owner:  owner,
		Repo:   repo,
		Number: number,
		URL: fmt.Sprintf(
			"https://github.com/%s/%s/pull/%d",
			owner,
			repo,
			number,
		),
	}
}

// ParseURL interpreta una URL de Pull Request de GitHub, por ejemplo:
// https://github.com/org/repo/pull/123
func ParseURL(rawURL string) (*PullRequest, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("no pude interpretar la URL")
	}

	if parsedURL.Host != "github.com" {
		return nil, fmt.Errorf("la URL tiene que pertenecer a github.com")
	}

	parts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")

	if len(parts) < 4 {
		return nil, fmt.Errorf("la URL no parece corresponder a un pull request")
	}

	if parts[2] != "pull" {
		return nil, fmt.Errorf("la URL no corresponde a un pull request")
	}

	number, err := strconv.Atoi(parts[3])
	if err != nil {
		return nil, fmt.Errorf("no pude identificar el número del pull request")
	}

	return &PullRequest{
		Owner:  parts[0],
		Repo:   parts[1],
		Number: number,
		URL:    rawURL,
	}, nil
}
