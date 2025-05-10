package main

import (
	"github.com/go-chi/chi/v5"
)

// routes returns a new chi router containing the application routes.
func (app *application) routes() *chi.Mux {
	r := chi.NewRouter()

	r.NotFound(app.notFoundResponse)
	r.MethodNotAllowed(app.methodNotAllowedResponse)

	r.Use(app.recoverPanic)

	r.Get("/healthcheck", app.healthcheckHandler)

	r.Post("/users", app.registerUserHandler)
	r.Post("/users/login", app.loginUserHandler)

	return r
}
