package healthcheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadHealthProfiles_DefaultsWhenNoFile(t *testing.T) {
	tmpDir := t.TempDir()

	profiles, err := LoadHealthProfiles(tmpDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(profiles.Profiles) != 4 {
		t.Errorf("Expected 4 default profiles, got %d", len(profiles.Profiles))
	}

	for _, name := range []string{"development", "production", "ci", "staging"} {
		if _, exists := profiles.Profiles[name]; !exists {
			t.Errorf("Expected default profile %q to exist", name)
		}
	}
}

func TestGetProfile(t *testing.T) {
	profiles := getDefaultProfiles()

	profile, err := profiles.GetProfile("production")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if profile.Name != "production" {
		t.Errorf("Expected name 'production', got '%s'", profile.Name)
	}
	if !profile.CircuitBreaker {
		t.Error("Expected circuit breaker to be enabled for production")
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	profiles := getDefaultProfiles()

	_, err := profiles.GetProfile("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent profile")
	}
}

func TestSaveSampleProfiles(t *testing.T) {
	tmpDir := t.TempDir()

	err := SaveSampleProfiles(tmpDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	profilePath := filepath.Join(tmpDir, ".azd", "health-profiles.yaml")
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Error("Expected profile file to be created")
	}

	// Second call should fail since file exists
	err = SaveSampleProfiles(tmpDir)
	if err == nil {
		t.Error("Expected error when file already exists")
	}
}
