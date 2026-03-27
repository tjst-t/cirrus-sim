// Package postgres provides an embedded PostgreSQL server for cirrus-sim.
package postgres

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

// Config holds configuration for the embedded PostgreSQL server.
type Config struct {
	Port     uint32
	Database string
	Username string
	Password string
	DataDir  string // empty = use temp dir (deleted on stop)
	Logger   io.Writer
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Port:     5432,
		Database: "cirrus",
		Username: "cirrus",
		Password: "cirrus",
	}
}

// Server wraps an embedded PostgreSQL instance.
type Server struct {
	db         *embeddedpostgres.EmbeddedPostgres
	config     Config
	connURL    string
	tmpDataDir string // non-empty if we created a temp dir
	logger     *slog.Logger
}

// New creates a new embedded PostgreSQL server.
func New(port string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	cfg := DefaultConfig()

	// Parse port
	var p uint32
	if _, err := fmt.Sscanf(port, "%d", &p); err == nil && p > 0 {
		cfg.Port = p
	}

	// Check for env overrides
	if v := os.Getenv("POSTGRES_DATABASE"); v != "" {
		cfg.Database = v
	}
	if v := os.Getenv("POSTGRES_USERNAME"); v != "" {
		cfg.Username = v
	}
	if v := os.Getenv("POSTGRES_PASSWORD"); v != "" {
		cfg.Password = v
	}
	if v := os.Getenv("POSTGRES_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}

	return &Server{
		config: cfg,
		logger: logger,
	}
}

// Start starts the embedded PostgreSQL server.
func (s *Server) Start() {
	dataDir := s.config.DataDir
	if dataDir == "" {
		tmpDir, err := os.MkdirTemp("", "cirrus-sim-pg-*")
		if err != nil {
			s.logger.Error("failed to create temp data dir", "error", err)
			return
		}
		dataDir = filepath.Join(tmpDir, "data")
		s.tmpDataDir = tmpDir
	}

	logWriter := s.config.Logger
	if logWriter == nil {
		logWriter = io.Discard
	}

	epConfig := embeddedpostgres.DefaultConfig().
		Port(s.config.Port).
		Database(s.config.Database).
		Username(s.config.Username).
		Password(s.config.Password).
		DataPath(dataDir).
		StartTimeout(60 * time.Second).
		Logger(logWriter)

	s.connURL = epConfig.GetConnectionURL()
	s.db = embeddedpostgres.NewDatabase(epConfig)

	s.logger.Info("postgres starting",
		"port", s.config.Port,
		"database", s.config.Database,
		"user", s.config.Username,
		"data_dir", dataDir,
	)

	if err := s.db.Start(); err != nil {
		s.logger.Error("postgres failed to start", "error", err)
		return
	}

	s.logger.Info("postgres ready", "url", s.connURL)
}

// Shutdown stops the embedded PostgreSQL server.
func (s *Server) Shutdown(_ context.Context) error {
	if s.db == nil {
		return nil
	}

	s.logger.Info("postgres stopping")
	if err := s.db.Stop(); err != nil {
		return fmt.Errorf("stop postgres: %w", err)
	}

	// Clean up temp dir if we created one
	if s.tmpDataDir != "" {
		os.RemoveAll(s.tmpDataDir)
	}

	s.logger.Info("postgres stopped")
	return nil
}

// ConnectionURL returns the PostgreSQL connection URL.
func (s *Server) ConnectionURL() string {
	return s.connURL
}
