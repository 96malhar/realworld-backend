package main

import (
	"github.com/go-chi/chi/v5"
)

// routes returns a new chi router containing the application routes.
func (app *application) routes() *chi.Mux {
	r := chi.NewRouter()

	r.NotFound(app.notFoundResponse)
	r.MethodNotAllowed(app.methodNotAllowedResponse)

	r.Use(app.recoverPanic, app.authenticate)

	r.Get("/healthcheck", app.healthcheckHandler)

	r.Post("/users", app.registerUserHandler)
	r.Post("/users/login", app.loginUserHandler)
	r.With(app.requireAuthenticatedUser).Get("/user", app.getCurrentUserHandler)

	r.Route("/profiles/{username}", func(r chi.Router) {
		r.Get("/", app.getProfileHandler)
		r.With(app.requireAuthenticatedUser).Post("/follow", app.followUserHandler)
		r.With(app.requireAuthenticatedUser).Delete("/follow", app.unfollowUserHandler)
	})

	return r
}
