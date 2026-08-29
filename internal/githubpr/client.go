package githubpr

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Info son los metadatos del PR que se muestran en el header de la TUI.
type Info struct {
	Title        string `json:"title"`
	State        string `json:"state"`
	BaseRefName  string `json:"baseRefName"`
	HeadRefName  string `json:"headRefName"`
	HeadRefOid   string `json:"headRefOid"`
	ChangedFiles int    `json:"changedFiles"`
	Additions    int    `json:"additions"`
	Deletions    int    `json:"deletions"`

	Author struct {
		Login string `json:"login"`
	} `json:"author"`
}

// ErrGHNotInstalled se devuelve cuando el binario `gh` no está disponible.
var ErrGHNotInstalled = fmt.Errorf(
	"no encontré el comando \"gh\" (GitHub CLI). Instalalo desde https://cli.github.com y autenticate con \"gh auth login\"",
)

func ghAvailable() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return ErrGHNotInstalled
	}

	return nil
}

// FetchInfo obtiene los metadatos del Pull Request usando `gh pr view`.
func FetchInfo(pr PullRequest) (*Info, error) {
	if err := ghAvailable(); err != nil {
		return nil, err
	}

	cmd := exec.Command(
		"gh",
		"pr",
		"view",
		strconv.Itoa(pr.Number),
		"--repo",
		pr.Repository(),
		"--json",
		"title,state,author,baseRefName,headRefName,headRefOid,changedFiles,additions,deletions",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf(
			"no pude obtener la información del pull request: %s",
			strings.TrimSpace(string(output)),
		)
	}

	var info Info

	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf(
			"GitHub respondió, pero no pude interpretar los datos del PR: %w",
			err,
		)
	}

	return &info, nil
}

// FetchDiff obtiene el diff completo del Pull Request usando `gh pr diff`.
func FetchDiff(pr PullRequest) (string, error) {
	if err := ghAvailable(); err != nil {
		return "", err
	}

	cmd := exec.Command(
		"gh",
		"pr",
		"diff",
		strconv.Itoa(pr.Number),
		"--repo",
		pr.Repository(),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf(
			"no pude obtener el código modificado: %s",
			strings.TrimSpace(string(output)),
		)
	}

	return string(output), nil
}
