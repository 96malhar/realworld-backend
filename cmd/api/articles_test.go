package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestCreateArticleHandler(t *testing.T) {
	t.Parallel()

	requestUrlPath := "/articles"
	ts := newTestServer(t)
	registerUser(t, ts, "bob", "bob@example.com", "password123")
	bobToken := loginUser(t, ts, "bob@example.com", "password123")
	authHeader := map[string]string{"Authorization": "Token " + bobToken}

	validRequestBody := `{
		"article": {
			"title": "Test Article",
			"description": "Test description",
			"body": "Test body content",
			"tagList": ["test", "golang"]
		}
	}`

	testcases := []handlerTestcase{
		{
			name:                   "Valid article creation",
			requestMethodType:      http.MethodPost,
			requestUrlPath:         requestUrlPath,
			requestHeader:          authHeader,
			requestBody:            validRequestBody,
			wantResponseStatusCode: http.StatusCreated,
			additionalChecks: func(t *testing.T, res *http.Response) {
				locationHeader := res.Header.Get("Location")
				if !strings.Contains(locationHeader, "/articles/test-article") {
					t.Errorf("expected Location header to contain /articles/test-article, got %s", locationHeader)
				}
			},
		},
		{
			name:              "Empty title validation error",
			requestMethodType: http.MethodPost,
			requestUrlPath:    requestUrlPath,
			requestHeader:     authHeader,
			requestBody: `{
			"article": {
				"title": "",
				"description": "Test description",
				"body": "Test body content",
				"tagList": ["test"]
				}
			}`,
			wantResponseStatusCode: http.StatusUnprocessableEntity,
		},
		{
			name:                   "Unauthorized user",
			requestMethodType:      http.MethodPost,
			requestUrlPath:         requestUrlPath,
			requestBody:            validRequestBody,
			wantResponseStatusCode: http.StatusUnauthorized,
		},
		{
			name:                   "Get method not allowed",
			requestMethodType:      http.MethodGet,
			requestUrlPath:         requestUrlPath,
			wantResponseStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name:              "Duplicate tags",
			requestMethodType: http.MethodPost,
			requestUrlPath:    requestUrlPath,
			requestHeader:     authHeader,
			requestBody: `{
			"article": {
				"title": "Another Article",
				"description": "Test description",
				"body": "Test body content",
				"tagList": ["test", "test", "golang"]
				}
			}`,
			wantResponseStatusCode: http.StatusUnprocessableEntity,
			wantResponse: errorResponse{
				Errors: []string{"TagList must not contain duplicate tags"},
			},
		},
		{
			name:              "malformed JSON",
			requestMethodType: http.MethodPost,
			requestUrlPath:    requestUrlPath,
			requestHeader:     authHeader,
			requestBody: `{
			"article": {
				"title": "Another Article",
				"description": "Test description",
				"body": "Test body content",
				"tagList": ["test", "test", "golang"]
				`, // missing closing braces
			wantResponseStatusCode: http.StatusBadRequest,
		},
		{
			name:                   "Wrong auth token",
			requestMethodType:      http.MethodPost,
			requestUrlPath:         requestUrlPath,
			requestHeader:          map[string]string{"Authorization": "Token wrong-token"},
			requestBody:            validRequestBody,
			wantResponseStatusCode: http.StatusUnauthorized,
		},
	}

	testHandler(t, ts, testcases...)
}
