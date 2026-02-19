package projects

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func toSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func CreateProject(ctx context.Context, pool *pgxpool.Pool, name, description string) (*Project, error) {
	id := toSlug(name)
	if id == "" {
		return nil, fmt.Errorf("invalid project name")
	}

	now := time.Now()
	defaultSettings := json.RawMessage(`{}`)
	_, err := pool.Exec(ctx,
		"INSERT INTO projects (id, name, description, settings, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)",
		id, name, description, defaultSettings, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("creating project: %w", err)
	}

	return &Project{ID: id, Name: name, Description: description, Settings: defaultSettings, CreatedAt: now, UpdatedAt: now}, nil
}

func ListProjects(ctx context.Context, pool *pgxpool.Pool) ([]Project, error) {
	rows, err := pool.Query(ctx, "SELECT id, name, description, settings, created_at, updated_at FROM projects ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Settings, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, nil
}

func GetProject(ctx context.Context, pool *pgxpool.Pool, id string) (*Project, error) {
	var p Project
	err := pool.QueryRow(ctx,
		"SELECT id, name, description, settings, created_at, updated_at FROM projects WHERE id = $1", id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Settings, &p.CreatedAt, &p.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}
	return &p, nil
}

func UpdateProject(ctx context.Context, pool *pgxpool.Pool, id, name, description string) (*Project, error) {
	now := time.Now()
	tag, err := pool.Exec(ctx,
		"UPDATE projects SET name = $1, description = $2, updated_at = $3 WHERE id = $4",
		name, description, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("updating project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, nil
	}
	return GetProject(ctx, pool, id)
}

func UpdateProjectSettings(ctx context.Context, pool *pgxpool.Pool, id string, settings json.RawMessage) (*Project, error) {
	now := time.Now()
	tag, err := pool.Exec(ctx,
		"UPDATE projects SET settings = $1, updated_at = $2 WHERE id = $3",
		settings, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("updating project settings: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, nil
	}
	return GetProject(ctx, pool, id)
}

func DeleteProject(ctx context.Context, pool *pgxpool.Pool, id string) error {
	_, err := pool.Exec(ctx, "DELETE FROM projects WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting project: %w", err)
	}
	return nil
}

func AddSource(ctx context.Context, pool *pgxpool.Pool, projectID, path, sourceType string, isCode bool, alias string) (*ProjectSource, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("path does not exist: %s", path)
	}

	id := fmt.Sprintf("%s/%s", projectID, toSlug(alias))
	if alias == "" {
		// Use the last path segment as alias
		parts := strings.Split(strings.TrimRight(path, "/"), "/")
		alias = parts[len(parts)-1]
		id = fmt.Sprintf("%s/%s", projectID, toSlug(alias))
	}

	now := time.Now()
	_, err := pool.Exec(ctx,
		"INSERT INTO project_sources (id, project_id, path, source_type, is_code, alias, added_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		id, projectID, path, sourceType, isCode, alias, now,
	)
	if err != nil {
		return nil, fmt.Errorf("adding source: %w", err)
	}

	return &ProjectSource{
		ID: id, ProjectID: projectID, Path: path, SourceType: sourceType,
		IsCode: isCode, Alias: alias, AddedAt: now,
	}, nil
}

func RemoveSource(ctx context.Context, pool *pgxpool.Pool, projectID, sourceID string) error {
	_, err := pool.Exec(ctx,
		"DELETE FROM project_sources WHERE id = $1 AND project_id = $2",
		sourceID, projectID,
	)
	if err != nil {
		return fmt.Errorf("removing source: %w", err)
	}
	return nil
}

// DetectProjectByPath finds the project whose source path best matches the given directory.
// It checks if the given path starts with any project_sources.path (longest match wins).
func DetectProjectByPath(ctx context.Context, pool *pgxpool.Pool, dirPath string) (*Project, *ProjectSource, error) {
	// Normalize: strip trailing slash
	dirPath = strings.TrimRight(dirPath, "/")

	rows, err := pool.Query(ctx,
		`SELECT ps.id, ps.project_id, ps.path, ps.source_type, ps.is_code, ps.alias,
		        ps.last_indexed_commit, ps.last_indexed_branch, ps.last_indexed_at, ps.added_at
		 FROM project_sources ps
		 ORDER BY LENGTH(ps.path) DESC`,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("querying sources: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s ProjectSource
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Path, &s.SourceType, &s.IsCode, &s.Alias,
			&s.LastIndexedCommit, &s.LastIndexedBranch, &s.LastIndexedAt, &s.AddedAt); err != nil {
			return nil, nil, fmt.Errorf("scanning source: %w", err)
		}

		sourcePath := strings.TrimRight(s.Path, "/")
		// Exact match or dirPath is a subdirectory of the source
		if dirPath == sourcePath || strings.HasPrefix(dirPath, sourcePath+"/") {
			p, err := GetProject(ctx, pool, s.ProjectID)
			if err != nil {
				return nil, nil, fmt.Errorf("getting project: %w", err)
			}
			if p == nil {
				continue
			}
			return p, &s, nil
		}
	}

	return nil, nil, nil
}

func ListSources(ctx context.Context, pool *pgxpool.Pool, projectID string) ([]ProjectSource, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, project_id, path, source_type, is_code, alias,
		        last_indexed_commit, last_indexed_branch, last_indexed_at, added_at
		 FROM project_sources WHERE project_id = $1 ORDER BY added_at DESC`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing sources: %w", err)
	}
	defer rows.Close()

	var sources []ProjectSource
	for rows.Next() {
		var s ProjectSource
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Path, &s.SourceType, &s.IsCode, &s.Alias,
			&s.LastIndexedCommit, &s.LastIndexedBranch, &s.LastIndexedAt, &s.AddedAt); err != nil {
			return nil, fmt.Errorf("scanning source: %w", err)
		}
		sources = append(sources, s)
	}
	return sources, nil
}
