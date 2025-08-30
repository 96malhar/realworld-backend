package main

import (
	"net/http"
	"strconv"
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

func TestFavoriteArticleHandler_PositiveCases(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	// Setup users
	registerUser(t, ts, "alice", "alice@example.com", "password123")
	registerUser(t, ts, "bob", "bob@example.com", "password123")
	aliceToken := loginUser(t, ts, "alice@example.com", "password123")
	bobToken := loginUser(t, ts, "bob@example.com", "password123")

	// Create an article by Alice
	location := createArticle(t, ts, aliceToken, "Test Article", "Test description", "Test body content", []string{"test", "golang"})
	slug := strings.TrimPrefix(location, "/articles/")

	// bob likes the article
	headers := map[string]string{"Authorization": "Token " + bobToken}
	res, err := ts.executeRequest(http.MethodPost, "/articles/"+slug+"/favorite", "", headers)
	require.NoError(t, err)
	defer res.Body.Close() // nolint: errcheck

	require.Equal(t, http.StatusOK, res.StatusCode)

	var response getArticleResponse
	readJsonResponse(t, res.Body, &response)

	assert.Equal(t, slug, response.Article.Slug)
	assert.Equal(t, "Test Article", response.Article.Title)
	assert.Equal(t, "Test description", response.Article.Description)
	assert.Equal(t, "Test body content", response.Article.Body)
	assert.ElementsMatch(t, []string{"test", "golang"}, response.Article.TagList)
	assert.WithinDuration(t, time.Now().UTC(), response.Article.CreatedAt, time.Second)
	assert.WithinDuration(t, time.Now().UTC(), response.Article.UpdatedAt, time.Second)
	assert.Equal(t, 1, response.Article.FavoritesCount)
	assert.True(t, response.Article.Favorited)
	assert.Equal(t, "alice", response.Article.Author.Username)

	// bob tries to like again - idempotent operation
	res, err = ts.executeRequest(http.MethodPost, "/articles/"+slug+"/favorite", "", headers)
	require.NoError(t, err)
	defer res.Body.Close() // nolint: errcheck

	require.Equal(t, http.StatusOK, res.StatusCode)

	var response2 getArticleResponse
	readJsonResponse(t, res.Body, &response2)

	assert.Equal(t, 1, response2.Article.FavoritesCount)
	assert.True(t, response2.Article.Favorited)

	// alice likes her own article
	headers = map[string]string{"Authorization": "Token " + aliceToken}
	res, err = ts.executeRequest(http.MethodPost, "/articles/"+slug+"/favorite", "", headers)
	require.NoError(t, err)
	defer res.Body.Close() // nolint: errcheck

	require.Equal(t, http.StatusOK, res.StatusCode)

	var response3 getArticleResponse
	readJsonResponse(t, res.Body, &response3)

	assert.Equal(t, 2, response3.Article.FavoritesCount)
	assert.True(t, response3.Article.Favorited)
}

func TestFavoriteArticleHandler_NegativeCases(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	// Setup users
	registerUser(t, ts, "alice", "alice@example.com", "password123")
	registerUser(t, ts, "bob", "bob@example.com", "password123")
	aliceToken := loginUser(t, ts, "alice@example.com", "password123")
	bobToken := loginUser(t, ts, "bob@example.com", "password123")

	// Create an article by Alice
	location := createArticle(t, ts, aliceToken, "Test Article", "Test description", "Test body content", []string{"test", "golang"})
	slug := strings.TrimPrefix(location, "/articles/")

	testcases := []handlerTestcase{
		{
			name:                   "Favorite non-existing article",
			requestMethodType:      http.MethodPost,
			requestUrlPath:         "/articles/non-existing-article/favorite",
			requestHeader:          map[string]string{"Authorization": "Token " + bobToken},
			wantResponseStatusCode: http.StatusNotFound,
		},
		{
			name:                   "Unauthorized user cannot favorite article",
			requestMethodType:      http.MethodPost,
			requestUrlPath:         "/articles/" + slug + "/favorite",
			wantResponseStatusCode: http.StatusUnauthorized,
		},
		{
			name:                   "Invalid token cannot favorite article",
			requestMethodType:      http.MethodPost,
			requestUrlPath:         "/articles/" + slug + "/favorite",
			requestHeader:          map[string]string{"Authorization": "Token invalid-token"},
			wantResponseStatusCode: http.StatusUnauthorized,
		},
		{
			name:                   "GET method not allowed",
			requestMethodType:      http.MethodGet,
			requestUrlPath:         "/articles/" + slug + "/favorite",
			requestHeader:          map[string]string{"Authorization": "Token " + bobToken},
			wantResponseStatusCode: http.StatusMethodNotAllowed,
		},
	}

	testHandler(t, ts, testcases...)
}

func TestUnfavoriteArticleHandler_PositiveCases(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	// Setup users
	registerUser(t, ts, "alice", "alice@example.com", "password123")
	registerUser(t, ts, "bob", "bob@example.com", "password123")
	aliceToken := loginUser(t, ts, "alice@example.com", "password123")
	bobToken := loginUser(t, ts, "bob@example.com", "password123")

	// Create an article by Alice
	location := createArticle(t, ts, aliceToken, "Test Article", "Test description", "Test body content", []string{"test", "golang"})
	slug := strings.TrimPrefix(location, "/articles/")

	// Bob likes the article first
	headers := map[string]string{"Authorization": "Token " + bobToken}
	res, err := ts.executeRequest(http.MethodPost, "/articles/"+slug+"/favorite", "", headers)
	require.NoError(t, err)
	defer res.Body.Close() // nolint: errcheck
	require.Equal(t, http.StatusOK, res.StatusCode)

	var response getArticleResponse
	readJsonResponse(t, res.Body, &response)
	assert.Equal(t, 1, response.Article.FavoritesCount)
	assert.True(t, response.Article.Favorited)

	// Now Bob unfavorites the article
	res, err = ts.executeRequest(http.MethodDelete, "/articles/"+slug+"/favorite", "", headers)
	require.NoError(t, err)
	defer res.Body.Close() // nolint: errcheck
	require.Equal(t, http.StatusOK, res.StatusCode)

	readJsonResponse(t, res.Body, &response)
	assert.Equal(t, slug, response.Article.Slug)
	assert.Equal(t, "Test Article", response.Article.Title)
	assert.Equal(t, 0, response.Article.FavoritesCount)
	assert.False(t, response.Article.Favorited)
	assert.Equal(t, "alice", response.Article.Author.Username)

	// Bob tries to unfavorite again - idempotent operation
	res, err = ts.executeRequest(http.MethodDelete, "/articles/"+slug+"/favorite", "", headers)
	require.NoError(t, err)
	defer res.Body.Close() // nolint: errcheck
	require.Equal(t, http.StatusOK, res.StatusCode)

	readJsonResponse(t, res.Body, &response)
	assert.Equal(t, 0, response.Article.FavoritesCount)
	assert.False(t, response.Article.Favorited)
}

func TestUnfavoriteArticleHandler_NegativeCases(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	// Setup users
	registerUser(t, ts, "alice", "alice@example.com", "password123")
	registerUser(t, ts, "bob", "bob@example.com", "password123")
	aliceToken := loginUser(t, ts, "alice@example.com", "password123")
	bobToken := loginUser(t, ts, "bob@example.com", "password123")

	// Create an article by Alice
	location := createArticle(t, ts, aliceToken, "Test Article", "Test description", "Test body content", []string{"test", "golang"})
	slug := strings.TrimPrefix(location, "/articles/")

	testcases := []handlerTestcase{
		{
			name:                   "Unfavorite non-existing article",
			requestMethodType:      http.MethodDelete,
			requestUrlPath:         "/articles/non-existing-article/favorite",
			requestHeader:          map[string]string{"Authorization": "Token " + bobToken},
			wantResponseStatusCode: http.StatusNotFound,
		},
		{
			name:                   "Unauthorized user cannot unfavorite article",
			requestMethodType:      http.MethodDelete,
			requestUrlPath:         "/articles/" + slug + "/favorite",
			wantResponseStatusCode: http.StatusUnauthorized,
		},
		{
			name:                   "Invalid token cannot unfavorite article",
			requestMethodType:      http.MethodDelete,
			requestUrlPath:         "/articles/" + slug + "/favorite",
			requestHeader:          map[string]string{"Authorization": "Token invalid-token"},
			wantResponseStatusCode: http.StatusUnauthorized,
		},
		{
			name:                   "GET method not allowed",
			requestMethodType:      http.MethodGet,
			requestUrlPath:         "/articles/" + slug + "/favorite",
			requestHeader:          map[string]string{"Authorization": "Token " + bobToken},
			wantResponseStatusCode: http.StatusMethodNotAllowed,
		},
	}

	testHandler(t, ts, testcases...)
}

func Test_Favorite_Unfavorite_ArticleHandler_Concurrency(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	// Create an article by a user
	registerUser(t, ts, "author", "author@example.com", "password123")
	authorToken := loginUser(t, ts, "author@example.com", "password123")
	location := createArticle(t, ts, authorToken, "Concurrent Unfavorite Test Article", "Test description", "Test body content", []string{"test"})
	slug := strings.TrimPrefix(location, "/articles/")

	// Create multiple users concurrently
	numUsers := 25
	userTokens := make([]string, numUsers)
	registrationErrs := make(chan error, numUsers)

	for i := 0; i < numUsers; i++ {
		go func(userIndex int) {
			defer func() {
				if r := recover(); r != nil {
					registrationErrs <- r.(error)
				}
			}()

			username := "unfavorite_user" + strconv.Itoa(userIndex+1)
			email := username + "@example.com"
			registerUser(t, ts, username, email, "password123")
			token := loginUser(t, ts, email, "password123")
			userTokens[userIndex] = token
			registrationErrs <- nil
		}(i)
	}

	for i := 0; i < numUsers; i++ {
		err := <-registrationErrs
		require.NoError(t, err)
	}

	// All users favorite the article
	favoriteErrs := make(chan error, numUsers)
	for _, token := range userTokens {
		go func(token string) {
			defer func() {
				if r := recover(); r != nil {
					favoriteErrs <- r.(error)
				}
			}()
			headers := map[string]string{"Authorization": "Token " + token}
			resp, err := ts.executeRequest(http.MethodPost, "/articles/"+slug+"/favorite", "", headers)
			if err != nil {
				favoriteErrs <- err
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				favoriteErrs <- assert.AnError
				return
			}
			favoriteErrs <- nil
		}(token)
	}

	for i := 0; i < numUsers; i++ {
		err := <-favoriteErrs
		require.NoError(t, err)
	}

	// Verify the favorites count
	resp, err := ts.executeRequest(http.MethodGet, "/articles/"+slug, "", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var response getArticleResponse
	readJsonResponse(t, resp.Body, &response)
	assert.Equal(t, numUsers, response.Article.FavoritesCount)

	// All users unfavorite the article concurrently
	unfavoriteErrs := make(chan error, numUsers)
	for _, token := range userTokens {
		go func(token string) {
			defer func() {
				if r := recover(); r != nil {
					unfavoriteErrs <- r.(error)
				}
			}()
			headers := map[string]string{"Authorization": "Token " + token}
			resp, err := ts.executeRequest(http.MethodDelete, "/articles/"+slug+"/favorite", "", headers)
			if err != nil {
				unfavoriteErrs <- err
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				unfavoriteErrs <- assert.AnError
				return
			}
			unfavoriteErrs <- nil
		}(token)
	}

	for i := 0; i < numUsers; i++ {
		err := <-unfavoriteErrs
		require.NoError(t, err)
	}

	// Verify the final favorites count is 0
	resp, err = ts.executeRequest(http.MethodGet, "/articles/"+slug, "", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	readJsonResponse(t, resp.Body, &response)
	assert.Equal(t, 0, response.Article.FavoritesCount)
}
