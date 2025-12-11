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

// SetSchema updates the active schema for subsequent queries.
func (s *SPCS) SetSchema(schema string) {
	s.cfg.Schema = schema
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

// ListComputePools runs SHOW COMPUTE POOLS and maps the results.
func (s *SPCS) ListComputePools(ctx context.Context) ([]models.ComputePool, error) {
	query := "SHOW COMPUTE POOLS"
	rows, err := s.client.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query compute pools: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("fetch columns: %w", err)
	}

	pools := []models.ComputePool{}
	for rows.Next() {
		rec, err := scanRowToMap(rows, cols)
		if err != nil {
			return nil, fmt.Errorf("scan compute pool row: %w", err)
		}
		pool := models.ComputePool{
			Name:           rec["name"],
			State:          strings.ToLower(fallback(rec["state"], rec["status"])),
			MinNodes:       rec["min_nodes"],
			MaxNodes:       rec["max_nodes"],
			InstanceFamily: rec["instance_family"],
		}
		if created := rec["created_on"]; created != "" {
			pool.CreatedAt = parseSnowflakeTime(created)
			pool.Age = models.HumanizeAge(pool.CreatedAt)
		}
		pools = append(pools, pool)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pools, nil
}

// ListImageRepositories runs SHOW IMAGE REPOSITORIES and maps the results.
func (s *SPCS) ListImageRepositories(ctx context.Context) ([]models.ImageRepository, error) {
	query := buildShowImageReposQuery(s.cfg)
	rows, err := s.client.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query image repositories: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("fetch columns: %w", err)
	}

	repos := []models.ImageRepository{}
	for rows.Next() {
		rec, err := scanRowToMap(rows, cols)
		if err != nil {
			return nil, fmt.Errorf("scan repo row: %w", err)
		}
		repo := models.ImageRepository{
			Name:          rec["name"],
			RepositoryURL: rec["repository_url"],
			Owner:         rec["owner"],
		}
		if created := rec["created_on"]; created != "" {
			repo.CreatedAt = parseSnowflakeTime(created)
			repo.Age = models.HumanizeAge(repo.CreatedAt)
		}
		repos = append(repos, repo)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return repos, nil
}

// DescribeService returns a key/value map from SHOW SERVICES LIKE.
func (s *SPCS) DescribeService(ctx context.Context, name string) (map[string]string, error) {
	query := buildShowServicesLikeQuery(s.cfg, name)
	rows, err := s.client.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("describe service: %w", err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("fetch columns: %w", err)
	}
	if rows.Next() {
		rec, err := scanRowToMap(rows, cols)
		if err != nil {
			return nil, fmt.Errorf("scan service row: %w", err)
		}
		return rec, nil
	}
	return map[string]string{}, nil
}

// ListServiceInstances runs SHOW SERVICE INSTANCES for a service.
func (s *SPCS) ListServiceInstances(ctx context.Context, name string) ([]models.ServiceInstance, error) {
	query := buildShowServiceInstancesQuery(s.cfg, name)
	rows, err := s.client.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query service instances: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("fetch columns: %w", err)
	}

	instances := []models.ServiceInstance{}
	for rows.Next() {
		rec, err := scanRowToMap(rows, cols)
		if err != nil {
			return nil, fmt.Errorf("scan instance row: %w", err)
		}
		inst := models.ServiceInstance{
			Name:   fallback(rec["name"], rec["instance_name"]),
			Status: strings.ToLower(fallback(rec["status"], rec["state"])),
			Node:   fallback(rec["node"], rec["host"]),
		}
		if created := rec["created_on"]; created != "" {
			inst.CreatedAt = parseSnowflakeTime(created)
			inst.Age = models.HumanizeAge(inst.CreatedAt)
		}
		instances = append(instances, inst)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return instances, nil
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

func buildShowServicesLikeQuery(cfg config.Config, name string) string {
	if cfg.Database != "" && cfg.Schema != "" {
		return fmt.Sprintf("SHOW SERVICES LIKE '%s' IN SCHEMA \"%s\".\"%s\"", name, cfg.Database, cfg.Schema)
	}
	if cfg.Schema != "" {
		return fmt.Sprintf("SHOW SERVICES LIKE '%s' IN SCHEMA \"%s\"", name, cfg.Schema)
	}
	return fmt.Sprintf("SHOW SERVICES LIKE '%s'", name)
}

func buildShowImageReposQuery(cfg config.Config) string {
	if cfg.Database != "" && cfg.Schema != "" {
		return fmt.Sprintf("SHOW IMAGE REPOSITORIES IN SCHEMA \"%s\".\"%s\"", cfg.Database, cfg.Schema)
	}
	if cfg.Schema != "" {
		return fmt.Sprintf("SHOW IMAGE REPOSITORIES IN SCHEMA \"%s\"", cfg.Schema)
	}
	return "SHOW IMAGE REPOSITORIES"
}

func buildShowServiceInstancesQuery(cfg config.Config, name string) string {
	if cfg.Database != "" && cfg.Schema != "" {
		return fmt.Sprintf("SHOW SERVICE INSTANCES IN SERVICE \"%s\".\"%s\".\"%s\"", cfg.Database, cfg.Schema, name)
	}
	if cfg.Schema != "" {
		return fmt.Sprintf("SHOW SERVICE INSTANCES IN SERVICE \"%s\".\"%s\"", cfg.Schema, name)
	}
	return fmt.Sprintf("SHOW SERVICE INSTANCES IN SERVICE \"%s\"", name)
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
