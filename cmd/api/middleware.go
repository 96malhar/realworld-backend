package main

import (
	"fmt"
	"github.com/96malhar/realworld-backend/internal/data"
	"net/http"
	"strings"
)

// recoverPanic recovers from a panic, logs the details, and sends a 500 internal server error response.
func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// authenticate checks the Authorization header and verifies the JWT.
// If the JWT is valid, it retrieves the user details based on the user ID and sets the user details in the request context using application.contextSetUser.
func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" || !strings.HasPrefix(header, "Token ") {
			// No token provided, proceed as anonymous
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		tokenString := strings.TrimPrefix(header, "Token ")
		claims, err := app.jwtMaker.VerifyToken(tokenString)
		if err != nil {
			// Token verification failed, proceed as anonymous
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		user, err := app.modelStore.Users.GetByID(claims.UserID)
		if err != nil {
			// User retrieval failed, proceed as anonymous
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		r = app.contextSetUser(r, user)
		next.ServeHTTP(w, r)
	})
}

// requireAuthenticatedUser checks if the user is authenticated.
// If not, it sends a 401 unauthorized response.
func (app *application) requireAuthenticatedUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)
		if user.IsAnonymous() {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}
