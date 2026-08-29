package githubpr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// ListTree devuelve las rutas de todos los archivos (blobs) del repositorio
// en el commit indicado. Repos muy grandes pueden truncar la respuesta de
// GitHub; en ese caso puede haber archivos que no aparezcan en el listado.
func ListTree(pr PullRequest, ref string) ([]string, error) {
	if err := ghAvailable(); err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("repos/%s/git/trees/%s?recursive=1", pr.Repository(), ref)

	cmd := exec.Command("gh", "api", endpoint)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf(
			"no pude listar los archivos del repositorio: %s",
			strings.TrimSpace(string(output)),
		)
	}

	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}

	if err := json.Unmarshal(output, &tree); err != nil {
		return nil, fmt.Errorf(
			"GitHub respondió, pero no pude interpretar el árbol de archivos: %w",
			err,
		)
	}

	paths := make([]string, 0, len(tree.Tree))

	for _, entry := range tree.Tree {
		if entry.Type == "blob" {
			paths = append(paths, entry.Path)
		}
	}

	return paths, nil
}

// DownloadFile obtiene el contenido crudo de un archivo puntual del
// repositorio en el commit indicado.
func DownloadFile(pr PullRequest, ref string, path string) ([]byte, error) {
	if err := ghAvailable(); err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf(
		"repos/%s/contents/%s?ref=%s",
		pr.Repository(),
		escapeContentPath(path),
		url.QueryEscape(ref),
	)

	cmd := exec.Command("gh", "api", "-H", "Accept: application/vnd.github.raw", endpoint)

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"no pude descargar %s: %s",
			path,
			strings.TrimSpace(stderr.String()),
		)
	}

	return stdout.Bytes(), nil
}

func escapeContentPath(path string) string {
	segments := strings.Split(path, "/")

	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}

	return strings.Join(segments, "/")
}
