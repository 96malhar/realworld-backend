package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type commentResponse struct {
	Comment comment `json:"comment"`
}

type comment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Author    profile   `json:"author"`
}

func TestCreateCommentHandler(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	// Setup: Register users and create an article
	registerUser(t, ts, "alice", "alice@example.com", "password123")
	registerUser(t, ts, "bob", "bob@example.com", "password123")
	aliceToken := loginUser(t, ts, "alice@example.com", "password123")
	bobToken := loginUser(t, ts, "bob@example.com", "password123")

	// Alice creates an article
	articleLocation := createArticle(t, ts, aliceToken, "How to Train Your Dragon", "Ever wonder how?", "It takes a Jacobian", []string{"dragons", "training"})

	validRequestBody := `{
		"comment": {
			"body": "His name was my name too."
		}
	}`

	testcases := []handlerTestcase{
		{
			name:                   "Valid comment creation",
			requestMethodType:      http.MethodPost,
			requestUrlPath:         articleLocation + "/comments",
			requestHeader:          map[string]string{"Authorization": "Token " + bobToken},
			requestBody:            validRequestBody,
			wantResponseStatusCode: http.StatusCreated,
			additionalChecks: func(t *testing.T, res *http.Response) {
				var resp commentResponse
				readJsonResponse(t, res.Body, &resp)

				now := time.Now()

				assert.Equal(t, "His name was my name too.", resp.Comment.Body)
				assert.Equal(t, "bob", resp.Comment.Author.Username)
				assert.False(t, false, "Bob does not follow himself")
				assert.NotZero(t, resp.Comment.ID)
				assert.WithinDuration(t, now, resp.Comment.CreatedAt, time.Second, "CreatedAt should be within 1 second of now")
				assert.WithinDuration(t, now, resp.Comment.UpdatedAt, time.Second, "UpdatedAt should be within 1 second of now")
			},
		},
		{
			name:              "Comment creation without authentication",
			requestMethodType: http.MethodPost,
			requestUrlPath:    articleLocation + "/comments",
			requestBody:       validRequestBody,
			// No auth header
			wantResponseStatusCode: http.StatusUnauthorized,
		},
		{
			name:                   "Comment creation with empty body",
			requestMethodType:      http.MethodPost,
			requestUrlPath:         articleLocation + "/comments",
			requestHeader:          map[string]string{"Authorization": "Token " + bobToken},
			requestBody:            `{"comment": {"body": ""}}`,
			wantResponseStatusCode: http.StatusUnprocessableEntity,
		},
		{
			name:                   "Comment creation with whitespace-only body",
			requestMethodType:      http.MethodPost,
			requestUrlPath:         articleLocation + "/comments",
			requestHeader:          map[string]string{"Authorization": "Token " + bobToken},
			requestBody:            `{"comment": {"body": "   "}}`,
			wantResponseStatusCode: http.StatusUnprocessableEntity,
		},
		{
			name:                   "Comment creation with missing body field",
			requestMethodType:      http.MethodPost,
			requestUrlPath:         articleLocation + "/comments",
			requestHeader:          map[string]string{"Authorization": "Token " + bobToken},
			requestBody:            `{"comment": {}}`,
			wantResponseStatusCode: http.StatusUnprocessableEntity,
		},
		{
			name:                   "Comment creation on non-existent article",
			requestMethodType:      http.MethodPost,
			requestUrlPath:         "/articles/non-existent-article-slug/comments",
			requestHeader:          map[string]string{"Authorization": "Token " + bobToken},
			requestBody:            validRequestBody,
			wantResponseStatusCode: http.StatusNotFound,
		},
		{
			name:                   "Comment creation with invalid JSON",
			requestMethodType:      http.MethodPost,
			requestUrlPath:         articleLocation + "/comments",
			requestHeader:          map[string]string{"Authorization": "Token " + bobToken},
			requestBody:            `{"comment": {"body": "test"`,
			wantResponseStatusCode: http.StatusBadRequest,
		},
	}

	testHandler(t, ts, testcases...)
}
