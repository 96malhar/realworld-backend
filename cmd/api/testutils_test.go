package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/96malhar/realworld-backend/internal/data"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type errorResponse struct {
	Errors []string `json:"errors"`
}

type testServer struct {
	router http.Handler
	app    *application
	db     *pgxpool.Pool
}

func newTestServer(t *testing.T) *testServer {
	testDb := newTestDB(t)
	app := &application{
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		config:     config{env: "development"},
		modelStore: data.NewModelStore(testDb),
	}

	return &testServer{
		router: app.routes(),
		app:    app,
		db:     testDb,
	}
}

func (ts *testServer) executeRequest(method, urlPath, body string, requestHeader map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(method, urlPath, strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	// convert requestHeader map to http.Header
	header := http.Header{}
	for key, val := range requestHeader {
		header.Add(key, val)
	}
	req.Header = header

	rr := httptest.NewRecorder()
	ts.router.ServeHTTP(rr, req)
	return rr.Result(), nil
}

func newTestDB(t *testing.T) *pgxpool.Pool {
	randomSuffix := strings.Split(uuid.New().String(), "-")[0]
	testDbName := fmt.Sprintf("testdb_%s", randomSuffix)
	rootDsn := "host=localhost port=5432 user=postgres password=postgres sslmode=disable"
	testDbDsn := fmt.Sprintf("%s dbname=%s", rootDsn, testDbName)

	rootDb := getDbConn(t, rootDsn)
	_, err := rootDb.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", testDbName))
	require.NoErrorf(t, err, "Failed to create database %s", testDbName)

	testDb := getDbConn(t, testDbDsn)
	t.Logf("Connected to database %s", testDbName)

	runMigrations(t, testDb, testDbName)

	t.Cleanup(func() {
		testDb.Close()
		_, err := rootDb.Exec(context.Background(), fmt.Sprintf("DROP DATABASE %s", testDbName))
		require.NoErrorf(t, err, "Failed to drop database %s", testDbName)
		t.Logf("Dropped database %s", testDbName)
		rootDb.Close()
	})

	return testDb
}

func getDbConn(t *testing.T, dsn string) *pgxpool.Pool {
	db, _ := pgxpool.New(context.Background(), dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := db.Ping(ctx)
	require.NoErrorf(t, err, "Failed to connect to postgres with DSN = %s", dsn)

	return db
}

func newTestApplication(db *pgxpool.Pool) *application {
	return &application{
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		config:     config{env: "development"},
		modelStore: data.NewModelStore(db),
	}
}

func runMigrations(t *testing.T, pool *pgxpool.Pool, dbName string) {
	t.Log("applying database migrations...")
	db := stdlib.OpenDBFromPool(pool)
	migrationDriver, err := pgx.WithInstance(db, &pgx.Config{})
	require.NoError(t, err)

	migrator, err := migrate.NewWithDatabaseInstance("file://../../migrations", dbName, migrationDriver)
	require.NoError(t, err)

	err = migrator.Up()
	require.NoError(t, err)

	t.Log("database migrations applied")
	migrator.Close()
	db.Close()
}

func readJsonResponse(t *testing.T, body io.Reader, dst any) {
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	require.NoError(t, err)
}
