package snowflake

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/yourusername/snow9s/internal/config"
	"github.com/yourusername/snow9s/pkg/models"
)

// SPCS provides Snowpark Container Services helpers.
type SPCS struct {
	client Queryable
	cfg    config.Config
}

// NewSPCS constructs the service wrapper.
func NewSPCS(client Queryable, cfg config.Config) *SPCS {
	return &SPCS{client: client, cfg: cfg}
}

// ListServices runs SHOW SERVICES and maps the results to Service models.
func (s *SPCS) ListServices(ctx context.Context) ([]models.Service, error) {
	query := buildShowServicesQuery(s.cfg)
	rows, err := s.client.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query services: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("fetch columns: %w", err)
	}

	services := []models.Service{}
	for rows.Next() {
		rec, err := scanRowToMap(rows, cols)
		if err != nil {
			return nil, fmt.Errorf("scan service row: %w", err)
		}

		service := models.Service{
			Name:        rec["name"],
			Namespace:   fallback(rec["schema_name"], s.cfg.Schema),
			Status:      strings.ToLower(fallback(rec["status"], rec["state"])),
			ComputePool: rec["compute_pool"],
		}

		if created := rec["created_on"]; created != "" {
			service.CreatedAt = parseSnowflakeTime(created)
			service.Age = models.HumanizeAge(service.CreatedAt)
		}

		services = append(services, service)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return services, nil
}

func buildShowServicesQuery(cfg config.Config) string {
	if cfg.Database != "" && cfg.Schema != "" {
		return fmt.Sprintf("SHOW SERVICES IN SCHEMA \"%s\".\"%s\"", cfg.Database, cfg.Schema)
	}
	if cfg.Schema != "" {
		return fmt.Sprintf("SHOW SERVICES IN SCHEMA \"%s\"", cfg.Schema)
	}
	return "SHOW SERVICES"
}

func scanRowToMap(rows *sql.Rows, cols []string) (map[string]string, error) {
	values := make([]sql.NullString, len(cols))
	ptrs := make([]any, len(cols))
	for i := range values {
		ptrs[i] = &values[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(cols))
	for i, col := range cols {
		if values[i].Valid {
			out[strings.ToLower(col)] = values[i].String
		}
	}
	return out, nil
}

func parseSnowflakeTime(raw string) time.Time {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999 -0700",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts
		}
	}
	return time.Time{}
}

func fallback(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
