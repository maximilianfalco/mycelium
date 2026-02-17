package integration

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/maximilianfalco/mycelium/internal/db"
	"github.com/maximilianfalco/mycelium/internal/projects"
)

func setupSettingsTest(t *testing.T) context.Context {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://mycelium:mycelium@localhost:5433/mycelium"
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	// Clean up any leftover test project
	_ = projects.DeleteProject(ctx, pool, "settings-test")

	t.Cleanup(func() {
		_ = projects.DeleteProject(ctx, pool, "settings-test")
	})

	return ctx
}

func TestCreateProject_DefaultSettings(t *testing.T) {
	ctx := setupSettingsTest(t)
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://mycelium:mycelium@localhost:5433/mycelium"
	}
	pool, _ := db.NewPool(ctx, dbURL)
	defer pool.Close()

	p, err := projects.CreateProject(ctx, pool, "Settings Test", "test project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if string(p.Settings) != "{}" {
		t.Errorf("expected default settings '{}', got %q", string(p.Settings))
	}
}

func TestUpdateProjectSettings(t *testing.T) {
	ctx := setupSettingsTest(t)
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://mycelium:mycelium@localhost:5433/mycelium"
	}
	pool, _ := db.NewPool(ctx, dbURL)
	defer pool.Close()

	_, err := projects.CreateProject(ctx, pool, "Settings Test", "test project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	newSettings := json.RawMessage(`{"maxFileSizeKB":200,"rootPath":"/tmp/test"}`)
	p, err := projects.UpdateProjectSettings(ctx, pool, "settings-test", newSettings)
	if err != nil {
		t.Fatalf("update settings: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(p.Settings, &got); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}

	if got["maxFileSizeKB"] != float64(200) {
		t.Errorf("expected maxFileSizeKB=200, got %v", got["maxFileSizeKB"])
	}
	if got["rootPath"] != "/tmp/test" {
		t.Errorf("expected rootPath=/tmp/test, got %v", got["rootPath"])
	}
}

func TestGetProject_IncludesSettings(t *testing.T) {
	ctx := setupSettingsTest(t)
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://mycelium:mycelium@localhost:5433/mycelium"
	}
	pool, _ := db.NewPool(ctx, dbURL)
	defer pool.Close()

	_, err := projects.CreateProject(ctx, pool, "Settings Test", "test project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	newSettings := json.RawMessage(`{"maxFileSizeKB":50}`)
	_, err = projects.UpdateProjectSettings(ctx, pool, "settings-test", newSettings)
	if err != nil {
		t.Fatalf("update settings: %v", err)
	}

	p, err := projects.GetProject(ctx, pool, "settings-test")
	if err != nil {
		t.Fatalf("get project: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(p.Settings, &got); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}

	if got["maxFileSizeKB"] != float64(50) {
		t.Errorf("expected maxFileSizeKB=50, got %v", got["maxFileSizeKB"])
	}
}

func TestUpdateProjectSettings_Overwrite(t *testing.T) {
	ctx := setupSettingsTest(t)
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://mycelium:mycelium@localhost:5433/mycelium"
	}
	pool, _ := db.NewPool(ctx, dbURL)
	defer pool.Close()

	_, err := projects.CreateProject(ctx, pool, "Settings Test", "test project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	first := json.RawMessage(`{"maxFileSizeKB":100,"rootPath":"/first"}`)
	_, err = projects.UpdateProjectSettings(ctx, pool, "settings-test", first)
	if err != nil {
		t.Fatalf("first update: %v", err)
	}

	second := json.RawMessage(`{"maxFileSizeKB":200}`)
	p, err := projects.UpdateProjectSettings(ctx, pool, "settings-test", second)
	if err != nil {
		t.Fatalf("second update: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(p.Settings, &got); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}

	if got["maxFileSizeKB"] != float64(200) {
		t.Errorf("expected maxFileSizeKB=200, got %v", got["maxFileSizeKB"])
	}
	if _, exists := got["rootPath"]; exists {
		t.Errorf("expected rootPath to be gone after overwrite, got %v", got["rootPath"])
	}
}
