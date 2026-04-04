package appmeta

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Release struct {
	Version    string   `json:"version"`
	ReleasedAt string   `json:"released_at"`
	Summary    string   `json:"summary"`
	Changes    []string `json:"changes"`
}

type releaseCatalog struct {
	Releases []Release `json:"releases"`
}

var (
	releaseCatalogOnce sync.Once
	cachedCatalog      releaseCatalog
	catalogErr         error
)

func Catalog() ([]Release, error) {
	releaseCatalogOnce.Do(func() {
		cachedCatalog, catalogErr = loadCatalog()
	})
	if catalogErr != nil {
		return nil, catalogErr
	}

	copyReleases := make([]Release, len(cachedCatalog.Releases))
	copy(copyReleases, cachedCatalog.Releases)
	return copyReleases, nil
}

func ValidateCatalog() error {
	releases, err := Catalog()
	if err != nil {
		return err
	}
	if len(releases) == 0 {
		return errors.New("release catalog is empty")
	}
	return nil
}

func CurrentRelease() (Release, error) {
	releases, err := Catalog()
	if err != nil {
		return Release{Version: Version}, err
	}
	for _, release := range releases {
		if release.Version == Version {
			return release, nil
		}
	}
	return Release{Version: Version}, fmt.Errorf("version %s is missing from release catalog", Version)
}

func loadCatalog() (releaseCatalog, error) {
	path, err := locateReleaseCatalog()
	if err != nil {
		return releaseCatalog{}, err
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return releaseCatalog{}, fmt.Errorf("read release catalog %s: %w", path, err)
	}

	var catalog releaseCatalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return releaseCatalog{}, fmt.Errorf("parse release catalog %s: %w", path, err)
	}
	if len(catalog.Releases) == 0 {
		return releaseCatalog{}, fmt.Errorf("release catalog %s is empty", path)
	}
	return catalog, nil
}

func locateReleaseCatalog() (string, error) {
	if envPath := os.Getenv("C2C_RELEASES_PATH"); envPath != "" {
		return envPath, nil
	}

	candidates := []string{
		"docs/releases.json",
		"../docs/releases.json",
		"../../docs/releases.json",
		"/app/docs/releases.json",
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates, filepath.Join(exeDir, "docs", "releases.json"))
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not locate docs/releases.json; checked %v", candidates)
}
