package main

import (
	"github.com/96malhar/realworld-backend/internal/auth"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

type userResponse struct {
	User user `json:"user"`
}

type user struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Image    string `json:"image"`
	Bio      string `json:"bio"`
	Token    string `json:"token"`
}

var seedUserRequest = `{
		"user": {
			"username": "Alice",
			"email": "alice@gmail.com",
			"password": "pa55word1234"
			}
		}`

func TestRegisterUserHandler(t *testing.T) {
	ts := newTestServer(t)

	// Insert a seed user
	res, err := ts.executeRequest(http.MethodPost,
		"/users", seedUserRequest, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, res.StatusCode)

	testCases := []struct {
		name                   string
		jwtMaker               *dummyJWTMaker
		requestBody            string
		wantResponseStatusCode int
		wantResponse           any
	}{
		{
			name:                   "Valid request",
			requestBody:            `{"user":{"username":"Bob", "email":"bob@gmail.com", "password":"pa55word1234"}}`,
			jwtMaker:               &dummyJWTMaker{},
			wantResponseStatusCode: http.StatusCreated,
			wantResponse: userResponse{
				User: user{
					Username: "Bob",
					Email:    "bob@gmail.com",
					Image:    "",
					Bio:      "",
					Token:    "dummy-token",
				},
			},
		},
		{
			name:                   "Invalid request body",
			requestBody:            `{"name":"Alice", "email":"alice@gmail.com", "password":"pa55word1234"}`,
			wantResponseStatusCode: http.StatusBadRequest,
			wantResponse: errorResponse{
				Errors: []string{"body contains unknown key \"name\""},
			},
		},
		{
			name:                   "Invalid email",
			requestBody:            `{"user":{"username":"Bob", "email":"bob.gmail.com", "password":"pa55word1234"}}`,
			wantResponseStatusCode: http.StatusUnprocessableEntity,
			wantResponse: errorResponse{
				Errors: []string{"email must be a valid email address"},
			},
		},
		{
			name:                   "Invalid password with empty username",
			requestBody:            `{"user":{"username":"", "email":"abc@gmail.com", "password":"123"}}`,
			wantResponseStatusCode: http.StatusUnprocessableEntity,
			wantResponse: errorResponse{
				Errors: []string{"username must be provided", "password must be at least 8 bytes long"},
			},
		},
		{
			name:                   "Duplicate email",
			requestBody:            `{"user":{"username":"alice_new", "email":"alice@gmail.com", "password":"pa55word1234"}}`,
			wantResponseStatusCode: http.StatusUnprocessableEntity,
			wantResponse: errorResponse{
				Errors: []string{"a user with this email address already exists"},
			},
		},
		{
			name:                   "Duplicate username",
			requestBody:            `{"user":{"username":"Alice", "email":"alice_new@gmail.com", "password":"pa55word1234"}}`,
			wantResponseStatusCode: http.StatusUnprocessableEntity,
			wantResponse: errorResponse{
				Errors: []string{"a user with this username already exists"},
			},
		},
		{
			name:                   "Badly formed request body with unclosed JSON",
			requestBody:            `{"username":"Bob", "email":`,
			wantResponseStatusCode: http.StatusBadRequest,
			wantResponse: errorResponse{
				Errors: []string{"body contains badly-formed JSON"},
			},
		},
		{
			name:                   "Badly formed request body",
			requestBody:            `{"user": {"username":"Bob", "email"}`,
			wantResponseStatusCode: http.StatusBadRequest,
			wantResponse: errorResponse{
				Errors: []string{"body contains badly-formed JSON (at character 36)"},
			},
		},
		{
			name:                   "Token creation error",
			jwtMaker:               &dummyJWTMaker{CreateTokenErr: auth.ErrInvalidToken},
			requestBody:            `{"user":{"username":"Bob2", "email":"bob2@gmail.com", "password":"pa55word1234"}}`,
			wantResponseStatusCode: http.StatusInternalServerError,
			wantResponse: errorResponse{
				Errors: []string{"the server encountered a problem and could not process your request"},
			},
		},
	}

	for _, tc := range testCases {
		if tc.jwtMaker != nil {
			ts.app.jwtMaker = tc.jwtMaker
		}
		testHandler(t, ts, handlerTestcase{
			name:                   tc.name,
			requestUrlPath:         "/users",
			requestMethodType:      http.MethodPost,
			requestBody:            tc.requestBody,
			wantResponseStatusCode: tc.wantResponseStatusCode,
			wantResponse:           tc.wantResponse,
		})
	}
}

func TestLoginUserHandler(t *testing.T) {
	ts := newTestServer(t)

	// Insert a seed user
	res, err := ts.executeRequest(http.MethodPost,
		"/users", seedUserRequest, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, res.StatusCode)

	testCases := []struct {
		name                   string
		jwtMaker               *dummyJWTMaker
		requestBody            string
		wantResponseStatusCode int
		wantResponse           any
	}{
		{
			name:                   "Valid request",
			requestBody:            `{"user":{"email":"alice@gmail.com", "password":"pa55word1234"}}`,
			jwtMaker:               &dummyJWTMaker{},
			wantResponseStatusCode: http.StatusOK,
			wantResponse: userResponse{
				User: user{
					Username: "Alice",
					Email:    "alice@gmail.com",
					Token:    "dummy-token",
					Image:    "",
					Bio:      "",
				},
			},
		},
		{
			name:                   "Email does not exist",
			requestBody:            `{"user":{"email":"alic@gmail.com", "password":"pa55word1234"}}`,
			wantResponseStatusCode: http.StatusUnauthorized,
			wantResponse: errorResponse{
				Errors: []string{"invalid authentication credentials"},
			},
		},
		{
			name:                   "Invalid password",
			requestBody:            `{"user":{"email":"alice@gmail.com", "password":"wrongpassword"}}`,
			wantResponseStatusCode: http.StatusUnauthorized,
			wantResponse: errorResponse{
				Errors: []string{"invalid authentication credentials"},
			},
		},
		{
			name:                   "Empty email and password",
			requestBody:            `{"user":{"email":"", "password":""}}`,
			wantResponseStatusCode: http.StatusUnprocessableEntity,
			wantResponse: errorResponse{
				Errors: []string{"email must be provided",
					"email must be a valid email address",
					"password must be provided",
					"password must be at least 8 bytes long",
				},
			},
		},
		{
			name:                   "Invalid request body",
			requestBody:            `{"user":{"name":"Alice", "password":"pa55word1234"}}`,
			wantResponseStatusCode: http.StatusBadRequest,
			wantResponse: errorResponse{
				Errors: []string{"body contains unknown key \"name\""},
			},
		},
		{
			name:                   "Token creation error",
			jwtMaker:               &dummyJWTMaker{CreateTokenErr: auth.ErrInvalidToken},
			requestBody:            `{"user":{"email":"alice@gmail.com", "password":"pa55word1234"}}`,
			wantResponseStatusCode: http.StatusInternalServerError,
			wantResponse: errorResponse{
				Errors: []string{"the server encountered a problem and could not process your request"},
			},
		},
	}

	for _, tc := range testCases {
		if tc.jwtMaker != nil {
			ts.app.jwtMaker = tc.jwtMaker
		}
		testHandler(t, ts, handlerTestcase{
			name:                   tc.name,
			requestUrlPath:         "/users/login",
			requestMethodType:      http.MethodPost,
			requestBody:            tc.requestBody,
			wantResponseStatusCode: tc.wantResponseStatusCode,
			wantResponse:           tc.wantResponse,
		})
	}
}
