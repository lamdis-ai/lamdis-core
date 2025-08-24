package adminapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type ConnectorSpec struct {
	ID           string `json:"id" yaml:"id"`
	Kind         string `json:"kind" yaml:"kind"`
	DisplayName  string `json:"display_name" yaml:"display_name"`
	Capabilities []struct {
		Canonical   string         `json:"canonical" yaml:"canonical"`
		Mode        string         `json:"mode" yaml:"mode"`
		Constraints map[string]any `json:"constraints,omitempty" yaml:"constraints,omitempty"`
	} `json:"capabilities" yaml:"capabilities"`
	Requirements struct {
		Secrets  []string `json:"secrets" yaml:"secrets"`
		Webhooks []string `json:"webhooks" yaml:"webhooks"`
	} `json:"requirements" yaml:"requirements"`
	AuditMode string `json:"audit_mode" yaml:"audit_mode"`
}

func loadRegistry(dir string) ([]ConnectorSpec, error) {
	if dir == "" {
		return nil, nil
	}
	out := []ConnectorSpec{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			return nil
		}

		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var spec ConnectorSpec
		if ext == ".json" {
			if err := json.Unmarshal(b, &spec); err != nil {
				return err
			}
		} else {
			if err := yaml.Unmarshal(b, &spec); err != nil {
				return fmt.Errorf("yaml parse: %w", err)
			}
		}
		if spec.ID != "" {
			out = append(out, spec)
		}
		return nil
	})
	return out, err
}

// seedDefaultConnector inserts a simple example connector if no connectors exist.
func seedDefaultConnector(ctx context.Context, db *pgxpool.Pool, log *zap.SugaredLogger) error {
	var n int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM connectors`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	type req struct {
		Secrets  []string `json:"secrets"`
		Webhooks []string `json:"webhooks"`
	}
	reqJSON, _ := json.Marshal(req{Secrets: []string{"api_key"}, Webhooks: []string{}})
	capJSON, _ := json.Marshal([]map[string]any{{
		"canonical": "orders.create",
		"mode":      "sync",
	}})
	_, err := db.Exec(ctx, `
		INSERT INTO connectors (id, kind, display_name, capabilities, requirements, audit_mode)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO NOTHING
	`, "sample", "sample", "Sample Connector", capJSON, reqJSON, "none")
	if err == nil {
		log.Infof("seeded default connector 'sample'")
	}
	return err
}

// importConnectorsFromDir loads specs from a directory and upserts them into DB.
func importConnectorsFromDir(ctx context.Context, db *pgxpool.Pool, log *zap.SugaredLogger, dir string) error {
	specs, err := loadRegistry(dir)
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		return nil
	}
	for _, s := range specs {
		if s.ID == "" {
			continue
		}
		if s.Kind == "" {
			s.Kind = s.ID
		}
		capb, _ := json.Marshal(s.Capabilities)
		reqb, _ := json.Marshal(s.Requirements)
		if _, err := db.Exec(ctx, `
			INSERT INTO connectors (id, kind, display_name, capabilities, requirements, audit_mode)
			VALUES ($1,$2,$3,$4,$5,COALESCE($6,'none'))
			ON CONFLICT (id) DO UPDATE SET
			  kind=EXCLUDED.kind,
			  display_name=EXCLUDED.display_name,
			  capabilities=EXCLUDED.capabilities,
			  requirements=EXCLUDED.requirements,
			  audit_mode=EXCLUDED.audit_mode,
			  updated_at=NOW()
		`, s.ID, s.Kind, s.DisplayName, capb, reqb, s.AuditMode); err != nil {
			return err
		}
	}
	log.Infof("imported %d connectors from %s", len(specs), dir)
	return nil
}
