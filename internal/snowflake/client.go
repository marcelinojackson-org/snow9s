package snowflake

import (
	"context"
	"database/sql"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"time"

	"crypto/rsa"
	"crypto/x509"

	"github.com/snowflakedb/gosnowflake"
	_ "github.com/snowflakedb/gosnowflake"
	"github.com/yourusername/snow9s/internal/config"
)

const defaultTimeout = 10 * time.Second

// Queryable abstracts sql.DB for easier testing.
type Queryable interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Client wraps the Snowflake connection.
type Client struct {
	db     *sql.DB
	debug  bool
	logger *log.Logger
}

// NewClient establishes a Snowflake connection and validates it with Ping.
func NewClient(ctx context.Context, cfg config.Config, logger *log.Logger) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	sfCfg := gosnowflake.Config{
		Account:   cfg.Account,
		User:      cfg.User,
		Warehouse: cfg.Warehouse,
		Database:  cfg.Database,
		Schema:    cfg.Schema,
	}
	if cfg.PrivateKeyPath != "" {
		keyBytes, err := os.ReadFile(cfg.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("read private key: %w", err)
		}
		privateKey, err := parseRSAPrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		sfCfg.PrivateKey = privateKey
		sfCfg.Authenticator = gosnowflake.AuthTypeJwt
	} else {
		sfCfg.Password = cfg.Password
	}

	dsn, err := gosnowflake.DSN(&sfCfg)
	if err != nil {
		return nil, fmt.Errorf("create DSN: %w", err)
	}

	db, err := sql.Open("snowflake", dsn)
	if err != nil {
		return nil, fmt.Errorf("open connection: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("ping Snowflake: %w", err)
	}

	if logger == nil {
		logger = log.New(log.Writer(), "snow9s", log.LstdFlags)
	}

	return &Client{db: db, debug: cfg.Debug, logger: logger}, nil
}

// QueryContext satisfies the Queryable interface while honoring debug logging.
func (c *Client) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return c.Query(ctx, query, args...)
}

// Query issues a SQL query with optional debug logging.
func (c *Client) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if c.debug {
		c.logger.Printf("SQL: %s", query)
	}
	return c.db.QueryContext(ctx, query, args...)
}

// Close releases the database connection.
func (c *Client) Close() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

// DB exposes the underlying handle when needed (e.g. tests).
func (c *Client) DB() *sql.DB {
	return c.db
}

func parseRSAPrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM data found")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("unexpected private key type %T", key)
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("unable to parse private key")
}
