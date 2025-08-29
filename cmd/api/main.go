package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/96malhar/realworld-backend/internal/auth"
	"github.com/96malhar/realworld-backend/internal/data"
	"github.com/96malhar/realworld-backend/internal/vcs"
	"github.com/jackc/pgx/v5/pgxpool"
)

var version = vcs.Version()

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxIdleTime  time.Duration
		maxOpenConns int
		timeout      time.Duration
	}
	jwtMaker struct {
		secretKey string
		issuer    string
	}
}

func (c config) LogValue() slog.Value {
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
	config     config
	logger     *slog.Logger
	modelStore data.ModelStore
	jwtMaker   jwtMaker
	wg         sync.WaitGroup
}

type envelope map[string]any

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := parseConfig()

	db, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		logger.Error("cannot connect to database", "dsn", cfg.db.dsn)
		os.Exit(1)
	}
	defer db.Close()

	app := &application{
		config:     cfg,
		logger:     logger,
		modelStore: data.NewModelStore(db, cfg.db.timeout),
		jwtMaker:   auth.NewJWTMaker(cfg.jwtMaker.secretKey, cfg.jwtMaker.issuer),
	}

	err = app.serve()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func parseConfig() config {
	var cfg config

	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("DB_DSN"), "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")
	flag.DurationVar(&cfg.db.timeout, "db-timeout", 5*time.Second, "PostgreSQL operation timeout")

	flag.StringVar(&cfg.jwtMaker.secretKey, "jwt-secret", os.Getenv("JWT_SECRET"), "JWT secret key")
	flag.StringVar(&cfg.jwtMaker.issuer, "jwt-issuer", os.Getenv("JWT_ISSUER"), "JWT issuer")

	// Create a new version boolean flag with the default value of false.
	displayVersion := flag.Bool("version", false, "Display version and exit")

	flag.Parse()

	if *displayVersion {
		fmt.Printf("Version:\t%s\n", version)
		os.Exit(0)
	}

	return cfg
}

func openDB(cfg config) (*pgxpool.Pool, error) {
	pgxConf, err := pgxpool.ParseConfig(cfg.db.dsn)
	if err != nil {
		return nil, err
	}
	pgxConf.MaxConnIdleTime = cfg.db.maxIdleTime
	pgxConf.MaxConns = int32(cfg.db.maxOpenConns)

	db, err := pgxpool.NewWithConfig(context.Background(), pgxConf)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.db.timeout)
	defer cancel()

	err = db.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return db, nil
}

type jwtMaker interface {
	CreateToken(userID int64, duration time.Duration) (string, error)
	VerifyToken(tokenString string) (*auth.Claims, error)
}
