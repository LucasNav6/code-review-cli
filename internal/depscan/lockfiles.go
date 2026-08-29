package depscan

// lockfileNames son los nombres de archivo de lockfiles/manifiestos que
// osv-scanner sabe interpretar. Cualquier otro archivo del repositorio se
// ignora: no hace falta bajar todo el árbol, solo lo que puede tener
// dependencias declaradas.
//
// companionManifest es un archivo hermano opcional que conviene bajar junto
// al lockfile (por ejemplo package.json junto a package-lock.json) porque
// algunos extractores lo usan para resolver metadata adicional.
var lockfileNames = map[string]string{
	"package-lock.json":   "package.json",
	"npm-shrinkwrap.json": "package.json",
	"yarn.lock":           "package.json",
	"pnpm-lock.yaml":      "package.json",
	"bun.lock":            "package.json",
	"composer.lock":       "composer.json",
	"go.sum":              "go.mod",
	"Gemfile.lock":        "Gemfile",
	"Cargo.lock":          "Cargo.toml",
	"requirements.txt":    "",
	"Pipfile.lock":        "Pipfile",
	"poetry.lock":         "pyproject.toml",
	"packages.lock.json":  "",
	"pubspec.lock":        "pubspec.yaml",
	"mix.lock":            "mix.exs",
}

func isKnownLockfile(basename string) bool {
	_, ok := lockfileNames[basename]

	return ok
}

func companionManifestFor(basename string) string {
	return lockfileNames[basename]
}
