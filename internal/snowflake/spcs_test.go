package snowflake

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/marcelinojackson-org/snow9s/internal/config"
)

func TestListServices(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	cfg := config.Config{Database: "DB", Schema: "PUBLIC"}
	rows := sqlmock.NewRows([]string{"created_on", "name", "schema_name", "status", "compute_pool"}).
		AddRow("2024-01-01 00:00:00 -0700", "svc1", "PUBLIC", "RUNNING", "pool1")

	mock.ExpectQuery("SHOW SERVICES IN SCHEMA \"DB\".\"PUBLIC\"").WillReturnRows(rows)

	spcs := NewSPCS(db, cfg)
	services, err := spcs.ListServices(context.Background())
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service got %d", len(services))
	}
	if services[0].Name != "svc1" || services[0].Status != "running" {
		t.Fatalf("unexpected service: %+v", services[0])
	}
	if services[0].Age == "" {
		t.Fatalf("age not set")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestListServicesEmpty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	cfg := config.Config{Schema: "PUBLIC"}
	rows := sqlmock.NewRows([]string{"name"})
	mock.ExpectQuery("SHOW SERVICES IN SCHEMA \"PUBLIC\"").WillReturnRows(rows)

	spcs := NewSPCS(db, cfg)
	services, err := spcs.ListServices(context.Background())
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(services) != 0 {
		t.Fatalf("expected empty list got %d", len(services))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// Integration test, skipped when credentials are absent.
func TestIntegrationSnowflakePing(t *testing.T) {
	required := []string{"SNOWFLAKE_ACCOUNT", "SNOWFLAKE_USER", "SNOWFLAKE_PASSWORD"}
	for _, env := range required {
		if os.Getenv(env) == "" {
			t.Skip("Snowflake credentials not provided; skipping integration test")
		}
	}
	cfg, err := config.LoadConfig("")
	if err != nil {
		t.Skip("unable to load config")
	}
	if err := cfg.Validate(); err != nil {
		t.Skip("invalid config")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := NewClient(ctx, cfg, nil)
	if err != nil {
		t.Fatalf("connect Snowflake: %v", err)
	}
	defer client.Close()

	if err := client.DB().PingContext(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
}
