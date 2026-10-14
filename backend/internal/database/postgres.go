package database

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"registre-admin/internal/config"
)

// slowQueryThreshold is the duration above which GORM logs a query.
const slowQueryThreshold = 500 * time.Millisecond

// NewPostgresDB opens the GORM connection with a bounded pool and a logger
// that never prints bound values: the registry stores health data (RAMQ,
// names) that must not leak into logs on a slow or failing query.
func NewPostgresDB(cfg config.DB) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: newGormLogger()})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	slog.Info("postgres connection established", "host", cfg.Host, "db", cfg.Name, "sslmode", cfg.SSLMode)
	return db, nil
}

// Close releases the connection pool; errors are logged, not returned,
// since it only runs at shutdown.
func Close(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		return
	}
	if err := sqlDB.Close(); err != nil {
		slog.Warn("closing postgres pool", "error", err)
	}
}

// newGormLogger routes GORM warnings to slog with parameterized SQL
// (placeholders instead of values) and ignores the expected
// ErrRecordNotFound noise.
func newGormLogger() gormlogger.Interface {
	return gormlogger.New(slogPrinter{}, gormlogger.Config{
		SlowThreshold:             slowQueryThreshold,
		LogLevel:                  gormlogger.Warn,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
	})
}

// slogPrinter adapts slog to the Printf-style writer GORM's logger expects.
type slogPrinter struct{}

func (slogPrinter) Printf(format string, args ...interface{}) {
	slog.Warn("gorm", "message", fmt.Sprintf(format, args...))
}
