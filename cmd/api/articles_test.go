package main

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type getArticleResponse struct {
	Article article `json:"article"`
}

type article struct {
	Slug           string    `json:"slug"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Body           string    `json:"body"`
	TagList        []string  `json:"tagList"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	FavoritesCount int       `json:"favoritesCount"`
	Favorited      bool      `json:"favorited"`
	Author         profile   `json:"author"`
}

func createArticle(t *testing.T, ts *testServer, token, title, description, body string, tags []string) string {
	t.Helper()

	requestBody := `{
		"article": {
			"title": "` + title + `",
			"description": "` + description + `",
			"body": "` + body + `",
			"tagList": [` + strings.Join(func() []string {
		var quotedTags []string
		for _, tag := range tags {
			quotedTags = append(quotedTags, `"`+tag+`"`)
		}
		return quotedTags
	}(), ",") + `]
		}
	}`

	headers := map[string]string{
		"Authorization": "Token " + token,
	}

	res, err := ts.executeRequest(http.MethodPost, "/articles", requestBody, headers)
	require.NoError(t, err)
	defer res.Body.Close() //nolint: errcheck

	require.Equal(t, http.StatusCreated, res.StatusCode)

	location := res.Header.Get("Location")
	require.NotEmpty(t, location, "Location header should not be empty")
	return location
}

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

	validRequestBodyNoTags := `{
		"article": {
			"title": "Test Article No Tags",
			"description": "Test description",
			"body": "Test body content"
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
			name:                   "Valid article creation with no tags",
			requestMethodType:      http.MethodPost,
			requestUrlPath:         requestUrlPath,
			requestHeader:          authHeader,
			requestBody:            validRequestBodyNoTags,
			wantResponseStatusCode: http.StatusCreated,
			additionalChecks: func(t *testing.T, res *http.Response) {
				locationHeader := res.Header.Get("Location")
				if !strings.Contains(locationHeader, "/articles/test-article-no-tags") {
					t.Errorf("expected Location header to contain /articles/test-article-no-tags, got %s", locationHeader)
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
			wantResponse: errorResponse{
				Errors: []string{"Title must not be empty or whitespace only"},
			},
		},
		{
			name:              "Missing Description validation error",
			requestMethodType: http.MethodPost,
			requestUrlPath:    requestUrlPath,
			requestHeader:     authHeader,
			requestBody: `{
			"article": {
				"title": "test-title",
				"body": "Test body content",
				"tagList": ["test"]
				}
			}`,
			wantResponseStatusCode: http.StatusUnprocessableEntity,
			wantResponse: errorResponse{
				Errors: []string{"Description must not be empty or whitespace only"},
			},
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
		{
			name:                   "Empty request body",
			requestMethodType:      http.MethodPost,
			requestUrlPath:         requestUrlPath,
			requestHeader:          authHeader,
			requestBody:            ``,
			wantResponseStatusCode: http.StatusBadRequest,
		},
		{
			name:                   "no auth token",
			requestMethodType:      http.MethodPost,
			requestUrlPath:         requestUrlPath,
			requestBody:            validRequestBody,
			wantResponseStatusCode: http.StatusUnauthorized,
		},
	}

	testHandler(t, ts, testcases...)
}

func TestGetArticleHandler(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)
	registerUser(t, ts, "alice", "alice@example.com", "password123")
	aliceToken := loginUser(t, ts, "alice@example.com", "password123")
	location := createArticle(t, ts, aliceToken, "Alice's Article", "Alice description", "Alice body content", []string{"alice", "golang"})

	testcases := []handlerTestcase{
		{
			name:                   "Get existing article",
			requestMethodType:      http.MethodGet,
			requestUrlPath:         location,
			wantResponseStatusCode: http.StatusOK,
			additionalChecks: func(t *testing.T, resp *http.Response) {
				var gotResponse getArticleResponse
				readJsonResponse(t, resp.Body, &gotResponse)
				assert.Equal(t, "Alice's Article", gotResponse.Article.Title)
				assert.Equal(t, "Alice description", gotResponse.Article.Description)
				assert.Equal(t, "Alice body content", gotResponse.Article.Body)
				assert.ElementsMatch(t, []string{"alice", "golang"}, gotResponse.Article.TagList)
				assert.WithinDuration(t, time.Now().UTC(), gotResponse.Article.CreatedAt, time.Second)
				assert.WithinDuration(t, time.Now().UTC(), gotResponse.Article.UpdatedAt, time.Second)
				assert.Equal(t, 0, gotResponse.Article.FavoritesCount)
				assert.False(t, gotResponse.Article.Favorited)
				assert.Equal(t, "alice", gotResponse.Article.Author.Username)
			},
		},
		{
			name:                   "Get non-existing article",
			requestMethodType:      http.MethodGet,
			requestUrlPath:         "/articles/non-existing-article",
			wantResponseStatusCode: http.StatusNotFound,
		},
	}

	testHandler(t, ts, testcases...)
}

func TestCreateArticleHandler_MultipleGoroutines(t *testing.T) {
	// create 3 articles concurrently
	t.Parallel()
	ts := newTestServer(t)
	registerUser(t, ts, "charlie", "charlie@example.com", "password123")
	charlieToken := loginUser(t, ts, "charlie@example.com", "password123")
	articleCount := 3
	locations := make([]string, articleCount)
	errs := make(chan error, articleCount)
	for i := 0; i < articleCount; i++ {
		go func(i int) {
			defer func() {
				if r := recover(); r != nil {
					errs <- r.(error)
				}
			}()
			loc := createArticle(t, ts, charlieToken, "Article "+string(rune('A'+i)), "Description "+string(rune('A'+i)), "Body content "+string(rune('A'+i)), []string{"tag" + string(rune('A'+i))})
			locations[i] = loc
			errs <- nil
		}(i)
	}

	for i := 0; i < articleCount; i++ {
		err := <-errs
		require.NoError(t, err)
	}
	require.Len(t, locations, articleCount)
	t.Logf("Created articles at locations: %v", locations)
}
