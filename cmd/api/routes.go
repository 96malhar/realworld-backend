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

	r.Route("/users", func(r chi.Router) {
		r.Post("/", app.registerUserHandler)
		r.Post("/login", app.loginUserHandler)
	})

	r.Route("/user", func(r chi.Router) {
		r.Use(app.requireAuthenticatedUser)
		r.Get("/", app.getCurrentUserHandler)
		r.Put("/", app.updateUserHandler)
	})

	r.Route("/profiles/{username}", func(r chi.Router) {
		r.Get("/", app.getProfileHandler)
		r.With(app.requireAuthenticatedUser).Post("/follow", app.followUserHandler)
		r.With(app.requireAuthenticatedUser).Delete("/follow", app.unfollowUserHandler)
	})

	r.Route("/articles", func(r chi.Router) {
		r.With(app.requireAuthenticatedUser).Post("/", app.createArticleHandler)
		r.Get("/{slug}", app.getArticleHandler)
	})

	return r
}
