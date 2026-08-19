package appmeta

import "testing"

func TestReleaseCatalogContainsCurrentVersion(t *testing.T) {
	if err := ValidateCatalog(); err != nil {
		t.Fatalf("ValidateCatalog returned error: %v", err)
	}

	release, err := CurrentRelease()
	if err != nil {
		t.Fatalf("CurrentRelease returned error: %v", err)
	}
	if release.Version != Version {
		t.Fatalf("expected current release %s, got %s", Version, release.Version)
	}
}
