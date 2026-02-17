package projects

import (
	"encoding/json"
	"time"
)

type Project struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Settings    json.RawMessage `json:"settings"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type ProjectSource struct {
	ID                string     `json:"id"`
	ProjectID         string     `json:"projectId"`
	Path              string     `json:"path"`
	SourceType        string     `json:"sourceType"`
	IsCode            bool       `json:"isCode"`
	Alias             string     `json:"alias"`
	LastIndexedCommit *string    `json:"lastIndexedCommit"`
	LastIndexedBranch *string    `json:"lastIndexedBranch"`
	LastIndexedAt     *time.Time `json:"lastIndexedAt"`
	AddedAt           time.Time  `json:"addedAt"`
}

type ScanResult struct {
	Path           string `json:"path"`
	Name           string `json:"name"`
	SourceType     string `json:"sourceType"`
	HasPackageJSON bool   `json:"hasPackageJson"`
}
