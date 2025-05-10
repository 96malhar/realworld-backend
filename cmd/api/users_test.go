package main

import (
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

	testCases := []handlerTestcase{
		{
			name:                   "Valid request",
			requestUrlPath:         "/users",
			requestMethodType:      http.MethodPost,
			requestBody:            `{"user":{"username":"Bob", "email":"bob@gmail.com", "password":"pa55word1234"}}`,
			wantResponseStatusCode: http.StatusCreated,
			wantResponse: userResponse{
				User: user{
					Username: "Bob",
					Email:    "bob@gmail.com",
					Image:    "",
					Bio:      "",
				},
			},
		},
		{
			name:                   "Invalid request body",
			requestUrlPath:         "/users",
			requestMethodType:      http.MethodPost,
			requestBody:            `{"name":"Alice", "email":"alice@gmail.com", "password":"pa55word1234"}`,
			wantResponseStatusCode: http.StatusBadRequest,
			wantResponse: errorResponse{
				Errors: []string{"body contains unknown key \"name\""},
			},
		},
		{
			name:                   "Invalid email",
			requestUrlPath:         "/users",
			requestMethodType:      http.MethodPost,
			requestBody:            `{"user":{"username":"Bob", "email":"bob.gmail.com", "password":"pa55word1234"}}`,
			wantResponseStatusCode: http.StatusUnprocessableEntity,
			wantResponse: errorResponse{
				Errors: []string{"email must be a valid email address"},
			},
		},
		{
			name:                   "Invalid password with empty username",
			requestUrlPath:         "/users",
			requestMethodType:      http.MethodPost,
			requestBody:            `{"user":{"username":"", "email":"abc@gmail.com", "password":"123"}}`,
			wantResponseStatusCode: http.StatusUnprocessableEntity,
			wantResponse: errorResponse{
				Errors: []string{"username must be provided", "password must be at least 8 bytes long"},
			},
		},
		{
			name:                   "Duplicate email",
			requestUrlPath:         "/users",
			requestMethodType:      http.MethodPost,
			requestBody:            `{"user":{"username":"Bob", "email":"alice@gmail.com", "password":"pa55word1234"}}`,
			wantResponseStatusCode: http.StatusUnprocessableEntity,
			wantResponse: errorResponse{
				Errors: []string{"a user with this email address already exists"},
			},
		},
		{
			name:                   "Badly formed request body with unclosed JSON",
			requestUrlPath:         "/users",
			requestMethodType:      http.MethodPost,
			requestBody:            `{"username":"Bob", "email":`,
			wantResponseStatusCode: http.StatusBadRequest,
			wantResponse: errorResponse{
				Errors: []string{"body contains badly-formed JSON"},
			},
		},
		{
			name:                   "Badly formed request body",
			requestUrlPath:         "/users",
			requestMethodType:      http.MethodPost,
			requestBody:            `{"user": {"username":"Bob", "email"}`,
			wantResponseStatusCode: http.StatusBadRequest,
			wantResponse: errorResponse{
				Errors: []string{"body contains badly-formed JSON (at character 36)"},
			},
		},
	}

	testHandler(t, ts, testCases...)
}
