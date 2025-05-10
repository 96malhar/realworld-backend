package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecoverPanic(t *testing.T) {
	app := newTestApplication(nil)
	router := chi.NewRouter()
	router.Use(app.recoverPanic)

	router.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
		panic("Oh no!")
	})

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/hello", nil))

	res := rr.Result()
	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	assert.Equal(t, res.Header.Get("Connection"), "close")
}
