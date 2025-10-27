package main

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/96malhar/realworld-backend/internal/auth"
	"github.com/96malhar/realworld-backend/internal/data"
	"github.com/jackc/pgx/v5/pgxpool"
)

type appConfig struct {
	port     int
	env      string
	db       dbConfig
	jwtMaker jwtMakerConfig
}

type dbConfig struct {
	dsn          string
	maxIdleTime  time.Duration
	maxOpenConns int
	timeout      time.Duration
}

type jwtMakerConfig struct {
	secretKey      string
	issuer         string
	accessDuration time.Duration
}

func (c appConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("port", c.port),
		slog.String("env", c.env),

		slog.Int("db-max-open-conns", c.db.maxOpenConns),
		slog.Duration("db-max-idle-time", c.db.maxIdleTime),
		slog.Duration("db-timeout", c.db.timeout),

		slog.String("version", version),
	)
}

type application struct {
	config     appConfig
	logger     *slog.Logger
	modelStore data.ModelStore
	jwtMaker   jwtMaker
	wg         sync.WaitGroup
}

type jwtMaker interface {
	CreateToken(userID int64, duration time.Duration) (string, error)
	VerifyToken(tokenString string) (*auth.Claims, error)
}

func newApplication(config appConfig, logger *slog.Logger) *application {
	jwtMaker, err := auth.NewJWTMaker(config.jwtMaker.secretKey, config.jwtMaker.issuer)
	if err != nil {
		slog.Error("failed to create JWT maker", "error", err)
		os.Exit(1)
	}

	return &application{
		config:     config,
		logger:     logger,
		modelStore: newModelStore(config),
		jwtMaker:   jwtMaker,
	}
}

func newModelStore(config appConfig) data.ModelStore {
	pgxConf, err := pgxpool.ParseConfig(config.db.dsn)
	if err != nil {
		slog.Error(err.Error())
		slog.Error("cannot parse database dsn", "dsn", config.db.dsn)
		os.Exit(1)
	}
	pgxConf.MaxConnIdleTime = config.db.maxIdleTime
	pgxConf.MaxConns = int32(config.db.maxOpenConns)

	db, err := pgxpool.NewWithConfig(context.Background(), pgxConf)
	if err != nil {
		slog.Error(err.Error())
		slog.Error("cannot connect to database", "dsn", config.db.dsn)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.db.timeout)
	defer cancel()

	err = db.Ping(ctx)
	if err != nil {
		slog.Error(err.Error())
		slog.Error("cannot ping database", "dsn", config.db.dsn)
		os.Exit(1)
	}

	return data.NewModelStore(db, config.db.timeout)
}
