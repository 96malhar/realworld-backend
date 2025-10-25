package main

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/96malhar/realworld-backend/internal/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type getArticleResponse struct {
	Article data.Article `json:"article"`
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

func TestDeleteArticleHandler(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	// 1. Create 2 users bob and alice
	registerUser(t, ts, "alice", "alice@example.com", "password123")
	registerUser(t, ts, "bob", "bob@example.com", "password123")
	aliceToken := loginUser(t, ts, "alice@example.com", "password123")
	bobToken := loginUser(t, ts, "bob@example.com", "password123")

	// 2. Create and article writte by bob
	location := createArticle(t, ts, bobToken, "Bob's Article", "Bob's description", "Bob's body content", []string{"bob"})
	slug := strings.TrimPrefix(location, "/articles/")

	// 3. Alice tries to delete the article and is not successful
	headers := map[string]string{"Authorization": "Token " + aliceToken}
	res, err := ts.executeRequest(http.MethodDelete, "/articles/"+slug, "", headers)
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusNotFound, res.StatusCode)

	// 4. Bob tries to delete it and is successful
	headers = map[string]string{"Authorization": "Token " + bobToken}
	res, err = ts.executeRequest(http.MethodDelete, "/articles/"+slug, "", headers)
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusNoContent, res.StatusCode)

	// Verify the article is actually deleted
	getRes, err := ts.executeRequest(http.MethodGet, "/articles/"+slug, "", nil)
	require.NoError(t, err)
	defer getRes.Body.Close()
	assert.Equal(t, http.StatusNotFound, getRes.StatusCode)

}

func TestUpdateArticleHandler(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	// Setup Alice
	registerUser(t, ts, "alice", "alice@example.com", "password123")
	aliceToken := loginUser(t, ts, "alice@example.com", "password123")

	// Setup Bob
	registerUser(t, ts, "bob", "bob@example.com", "password123")
	bobToken := loginUser(t, ts, "bob@example.com", "password123")

	// Create 2 articles by alice
	location1 := createArticle(t, ts, aliceToken, "Original Title", "Original description", "Original body content", []string{"original"})
	slug1 := strings.TrimPrefix(location1, "/articles/")
	location2 := createArticle(t, ts, aliceToken, "Second Article", "Second description", "Second body content", []string{"second"})
	slug2 := strings.TrimPrefix(location2, "/articles/")

	testcases := []handlerTestcase{
		{
			name:              "Update article successfully",
			requestMethodType: http.MethodPut,
			requestUrlPath:    "/articles/" + slug1,
			requestHeader:     map[string]string{"Authorization": "Token " + aliceToken},
			requestBody: `{
				"article": {
					"title": "Updated Title",
					"description": "Updated description",
					"body": "Updated body content"
				}
			}`,
			wantResponseStatusCode: http.StatusOK,
			additionalChecks: func(t *testing.T, resp *http.Response) {
				var gotResponse getArticleResponse
				readJsonResponse(t, resp.Body, &gotResponse)
				assert.Equal(t, "Updated Title", gotResponse.Article.Title)
				assert.Equal(t, "Updated description", gotResponse.Article.Description)
				assert.Equal(t, "Updated body content", gotResponse.Article.Body)
				assert.ElementsMatch(t, []string{"original"}, gotResponse.Article.TagList)
				assert.True(t, gotResponse.Article.UpdatedAt.After(gotResponse.Article.CreatedAt))
				assert.Equal(t, "alice", gotResponse.Article.Author.Username)
				locationHeader := resp.Header.Get("Location")
				assert.Contains(t, locationHeader, "/articles/updated-title")
			},
		},
		{
			name:              "Update article with empty title",
			requestMethodType: http.MethodPut,
			requestUrlPath:    "/articles/" + slug2,
			requestHeader:     map[string]string{"Authorization": "Token " + aliceToken},
			requestBody: `{
				"article": {
					"title": "",
					"description": "Updated description",
					"body": "Updated body content"
				}
			}`,
			wantResponseStatusCode: http.StatusUnprocessableEntity,
			wantResponse: errorResponse{
				Errors: []string{"Title must not be empty or whitespace only"},
			},
		},
		{
			name:              "Unauthorized user cannot update article",
			requestMethodType: http.MethodPut,
			requestUrlPath:    "/articles/" + slug2,
			requestBody: `{
				"article": {
					"title": "Updated Title",
					"description": "Updated description",
					"body": "Updated body content"
				}
			}`,
			wantResponseStatusCode: http.StatusUnauthorized,
		},
		{
			name:              "Bob cannot update Alice's article",
			requestMethodType: http.MethodPut,
			requestUrlPath:    "/articles/" + slug2,
			requestHeader:     map[string]string{"Authorization": "Token " + bobToken},
			requestBody: `{
				"article": {
					"title": "Updated Title",
					"description": "Updated description",
					"body": "Updated body content"
				}
			}`,
			wantResponseStatusCode: http.StatusForbidden,
		},
		{
			name:              "Update non-existing article",
			requestMethodType: http.MethodPut,
			requestUrlPath:    "/articles/non-existing-article",
			requestHeader:     map[string]string{"Authorization": "Token " + aliceToken},
			requestBody: `{
				"article": {
					"title": "Updated Title",
					"description": "Updated description",
					"body": "Updated body content"
				}
			}`,
			wantResponseStatusCode: http.StatusNotFound,
		},
	}

	testHandler(t, ts, testcases...)
}

func TestListArticlesHandler(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	// Create 7 test users
	registerUser(t, ts, "alice", "alice@example.com", "password123")
	registerUser(t, ts, "bob", "bob@example.com", "password123")
	registerUser(t, ts, "charlie", "charlie@example.com", "password123")
	registerUser(t, ts, "david", "david@example.com", "password123")
	registerUser(t, ts, "emily", "emily@example.com", "password123")
	registerUser(t, ts, "frank", "frank@example.com", "password123")
	registerUser(t, ts, "grace", "grace@example.com", "password123")

	aliceToken := loginUser(t, ts, "alice@example.com", "password123")
	bobToken := loginUser(t, ts, "bob@example.com", "password123")
	charlieToken := loginUser(t, ts, "charlie@example.com", "password123")
	davidToken := loginUser(t, ts, "david@example.com", "password123")
	emilyToken := loginUser(t, ts, "emily@example.com", "password123")
	frankToken := loginUser(t, ts, "frank@example.com", "password123")
	graceToken := loginUser(t, ts, "grace@example.com", "password123")

	// Each user creates multiple articles with overlapping tags
	// Alice creates 3 articles (golang, tutorial, advanced)
	article1 := createArticle(t, ts, aliceToken, "Golang Basics", "Learn Go", "Content about Go", []string{"golang", "tutorial", "backend"})
	article2 := createArticle(t, ts, aliceToken, "Advanced Golang", "Advanced Go", "Advanced Go content", []string{"golang", "advanced", "backend"})
	_ = createArticle(t, ts, aliceToken, "Go Concurrency", "Master goroutines", "Deep dive into concurrency", []string{"golang", "concurrency", "advanced"})

	// Bob creates 2 articles (javascript, tutorial, web)
	_ = createArticle(t, ts, bobToken, "React Guide", "Learn React", "Content about React", []string{"react", "javascript", "web", "tutorial"})
	_ = createArticle(t, ts, bobToken, "React Hooks", "Master Hooks", "Understanding React Hooks", []string{"react", "hooks", "web", "advanced"})

	// Charlie creates 3 articles (python, web, tutorial, frontend)
	_ = createArticle(t, ts, charlieToken, "Python Tutorial", "Learn Python", "Python content", []string{"python", "tutorial", "backend"})
	_ = createArticle(t, ts, charlieToken, "Python Django", "Web with Django", "Building web apps", []string{"python", "django", "web", "backend"})
	_ = createArticle(t, ts, charlieToken, "Frontend Fundamentals", "HTML, CSS, JS", "Building beautiful UIs", []string{"frontend", "javascript", "web"})

	// David creates 3 articles (rust, tutorial, advanced, web)
	_ = createArticle(t, ts, davidToken, "Rust Basics", "Introduction to Rust", "Getting started with Rust", []string{"rust", "tutorial", "backend"})
	_ = createArticle(t, ts, davidToken, "Rust Ownership", "Understanding Ownership", "Memory safety in Rust", []string{"rust", "advanced", "backend"})
	_ = createArticle(t, ts, davidToken, "Rust WebAssembly", "Rust meets WASM", "Web development with Rust", []string{"rust", "wasm", "web"})

	// Emily creates 2 articles (javascript, advanced, web)
	_ = createArticle(t, ts, emilyToken, "TypeScript Guide", "Type-safe JavaScript", "Introduction to TypeScript", []string{"typescript", "javascript", "web", "tutorial"})
	_ = createArticle(t, ts, emilyToken, "Advanced TypeScript", "Generics and Types", "Advanced type system", []string{"typescript", "advanced", "web"})

	// Frank creates 2 articles (devops, tutorial, advanced)
	_ = createArticle(t, ts, frankToken, "Docker Basics", "Containerization", "Getting started with Docker", []string{"docker", "devops", "tutorial"})
	_ = createArticle(t, ts, frankToken, "Kubernetes Guide", "Orchestration", "Container orchestration", []string{"kubernetes", "devops", "advanced"})

	// Grace creates 2 articles (python, advanced, data science)
	_ = createArticle(t, ts, graceToken, "Data Science 101", "Introduction to DS", "Getting started with data", []string{"datascience", "python", "tutorial"})
	_ = createArticle(t, ts, graceToken, "Machine Learning", "ML Algorithms", "Understanding ML", []string{"datascience", "ml", "advanced"})

	// Setup favorites: Bob favorites Alice's golang articles
	slug1 := strings.TrimPrefix(article1, "/articles/")
	slug2 := strings.TrimPrefix(article2, "/articles/")
	favoriteArticleHelper(t, ts, bobToken, slug1)
	favoriteArticleHelper(t, ts, bobToken, slug2)

	// Setup complex follow relationships (multiple users following multiple people)
	// Charlie follows Alice, Bob, and David
	followUser(t, ts, charlieToken, "alice")
	followUser(t, ts, charlieToken, "bob")
	followUser(t, ts, charlieToken, "david")

	// David follows Alice, Bob, and Emily
	followUser(t, ts, davidToken, "alice")
	followUser(t, ts, davidToken, "bob")
	followUser(t, ts, davidToken, "emily")

	// Emily follows Alice, Charlie, and Frank
	followUser(t, ts, emilyToken, "alice")
	followUser(t, ts, emilyToken, "charlie")
	followUser(t, ts, emilyToken, "frank")

	// Frank follows Bob, David, and Grace
	followUser(t, ts, frankToken, "bob")
	followUser(t, ts, frankToken, "david")
	followUser(t, ts, frankToken, "grace")

	// Grace follows Alice, Emily, and Frank
	followUser(t, ts, graceToken, "alice")
	followUser(t, ts, graceToken, "emily")
	followUser(t, ts, graceToken, "frank")

	// Bob follows Alice and Charlie
	followUser(t, ts, bobToken, "alice")
	followUser(t, ts, bobToken, "charlie")

	// Alice follows Bob and David
	followUser(t, ts, aliceToken, "bob")
	followUser(t, ts, aliceToken, "david")

	t.Run("List all articles", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Token " + aliceToken}
		res, err := ts.executeRequest(http.MethodGet, "/articles", "", headers)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)

		var response struct {
			Articles      []data.Article `json:"articles"`
			ArticlesCount int            `json:"articlesCount"`
		}
		readJsonResponse(t, res.Body, &response)

		assert.Equal(t, 17, response.ArticlesCount, "Should have 17 total articles")
		assert.Len(t, response.Articles, 17, "Should return 17 articles")

		// Verify all articles have proper author info
		for _, article := range response.Articles {
			assert.NotEmpty(t, article.Author.Username, "Author username should be populated")
		}

		// Verify ordering (most recent first) - check first few
		assert.Equal(t, "Machine Learning", response.Articles[0].Title)
		assert.Equal(t, "Data Science 101", response.Articles[1].Title)
		assert.Equal(t, "Kubernetes Guide", response.Articles[2].Title)
	})

	t.Run("Unauthenticated request returns all articles with following and favorited as false", func(t *testing.T) {
		// Make request without Authorization header
		res, err := ts.executeRequest(http.MethodGet, "/articles", "", nil)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)

		var response struct {
			Articles      []data.Article `json:"articles"`
			ArticlesCount int            `json:"articlesCount"`
		}
		readJsonResponse(t, res.Body, &response)

		assert.Equal(t, 17, response.ArticlesCount, "Should have 17 total articles")
		assert.Len(t, response.Articles, 17, "Should return 17 articles")

		// Verify all articles have valid fields and following/favorited are false
		for _, article := range response.Articles {
			// Verify following and favorited are false for unauthenticated users
			assert.False(t, article.Favorited, "Unauthenticated user should have favorited=false")
			assert.False(t, article.Author.Following, "Unauthenticated user should have following=false")

			// Verify all other fields have valid values
			assert.NotEmpty(t, article.Slug, "Slug should not be empty")
			assert.NotEmpty(t, article.Title, "Title should not be empty")
			assert.NotEmpty(t, article.Description, "Description should not be empty")
			assert.Empty(t, article.Body, "Body should be empty in list results")
			assert.NotNil(t, article.TagList, "TagList should not be nil")
			assert.NotZero(t, article.CreatedAt, "CreatedAt should not be zero")
			assert.NotZero(t, article.UpdatedAt, "UpdatedAt should not be zero")
			assert.True(t, article.FavoritesCount >= 0, "FavoritesCount should be non-negative")
			assert.NotEmpty(t, article.Author.Username, "Author username should be populated")
		}
	})

	t.Run("Following status reflects user relationships", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Token " + charlieToken}
		res, err := ts.executeRequest(http.MethodGet, "/articles", "", headers)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)

		var response struct {
			Articles      []data.Article `json:"articles"`
			ArticlesCount int            `json:"articlesCount"`
		}
		readJsonResponse(t, res.Body, &response)

		assert.Equal(t, 17, response.ArticlesCount)
		assert.Len(t, response.Articles, 17)

		// Charlie follows Alice, Bob, and David
		for _, article := range response.Articles {
			switch article.Author.Username {
			case "alice", "bob", "david":
				assert.True(t, article.Author.Following, "Charlie follows %s", article.Author.Username)
			case "charlie":
				assert.False(t, article.Author.Following, "Charlie doesn't follow himself")
			default:
				assert.False(t, article.Author.Following, "Charlie doesn't follow %s", article.Author.Username)
			}
			assert.False(t, article.Favorited, "Charlie hasn't favorited any articles")
		}
	})

	t.Run("Filter by tag", func(t *testing.T) {
		testCases := []struct {
			name          string
			token         string
			tag           string
			expectedCount int
		}{
			{
				name:          "specific language golang",
				token:         aliceToken,
				tag:           "golang",
				expectedCount: 3,
			},
			{
				name:          "overlapping tag tutorial",
				token:         bobToken,
				tag:           "tutorial",
				expectedCount: 7,
			},
			{
				name:          "advanced articles",
				token:         charlieToken,
				tag:           "advanced",
				expectedCount: 7,
			},
			{
				name:          "web development",
				token:         davidToken,
				tag:           "web",
				expectedCount: 7,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				headers := map[string]string{"Authorization": "Token " + tc.token}
				res, err := ts.executeRequest(http.MethodGet, "/articles?tag="+tc.tag, "", headers)
				require.NoError(t, err)
				defer res.Body.Close()

				require.Equal(t, http.StatusOK, res.StatusCode)

				var response struct {
					Articles      []data.Article `json:"articles"`
					ArticlesCount int            `json:"articlesCount"`
				}
				readJsonResponse(t, res.Body, &response)

				assert.Equal(t, tc.expectedCount, response.ArticlesCount)
				assert.Len(t, response.Articles, tc.expectedCount)

				// Verify all articles have the expected tag
				for _, article := range response.Articles {
					assert.Contains(t, article.TagList, tc.tag)
				}
			})
		}
	})

	t.Run("Filter by author", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Token " + bobToken}
		res, err := ts.executeRequest(http.MethodGet, "/articles?author=alice", "", headers)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)

		var response struct {
			Articles      []data.Article `json:"articles"`
			ArticlesCount int            `json:"articlesCount"`
		}
		readJsonResponse(t, res.Body, &response)

		assert.Equal(t, 3, response.ArticlesCount, "Alice has 3 articles")
		assert.Len(t, response.Articles, 3)

		// Verify all articles are by Alice
		for _, article := range response.Articles {
			assert.Equal(t, "alice", article.Author.Username)
		}
	})

	t.Run("Filter by favorited user", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Token " + charlieToken}
		res, err := ts.executeRequest(http.MethodGet, "/articles?favorited=bob", "", headers)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)

		var response struct {
			Articles      []data.Article `json:"articles"`
			ArticlesCount int            `json:"articlesCount"`
		}
		readJsonResponse(t, res.Body, &response)

		assert.Equal(t, 2, response.ArticlesCount, "Bob favorited 2 articles")
		assert.Len(t, response.Articles, 2)

		// Verify these are the articles Bob favorited (most recent first)
		titles := []string{response.Articles[0].Title, response.Articles[1].Title}
		assert.Contains(t, titles, "Golang Basics")
		assert.Contains(t, titles, "Advanced Golang")

		// All favorited articles should be by Alice
		for _, article := range response.Articles {
			assert.Equal(t, "alice", article.Author.Username)
		}
	})

	t.Run("Favorited status reflects user favorites", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Token " + bobToken}
		res, err := ts.executeRequest(http.MethodGet, "/articles", "", headers)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)

		var response struct {
			Articles      []data.Article `json:"articles"`
			ArticlesCount int            `json:"articlesCount"`
		}
		readJsonResponse(t, res.Body, &response)

		assert.Equal(t, 17, response.ArticlesCount)

		// Check favorited status for each article
		favoritedCount := 0
		for _, article := range response.Articles {
			if article.Title == "Golang Basics" || article.Title == "Advanced Golang" {
				assert.True(t, article.Favorited, "%s should be favorited by Bob", article.Title)
				favoritedCount++
			} else {
				assert.False(t, article.Favorited, "%s should not be favorited by Bob", article.Title)
			}
		}
		assert.Equal(t, 2, favoritedCount, "Bob should have favorited exactly 2 articles")
	})

	t.Run("Following status for different users", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Token " + davidToken}
		res, err := ts.executeRequest(http.MethodGet, "/articles", "", headers)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)

		var response struct {
			Articles      []data.Article `json:"articles"`
			ArticlesCount int            `json:"articlesCount"`
		}
		readJsonResponse(t, res.Body, &response)

		assert.Equal(t, 17, response.ArticlesCount)

		// Check following status - David follows Alice, Bob, and Emily
		for _, article := range response.Articles {
			if article.Author.Username == "alice" || article.Author.Username == "bob" || article.Author.Username == "emily" {
				assert.True(t, article.Author.Following, "David follows %s", article.Author.Username)
			} else {
				assert.False(t, article.Author.Following, "David doesn't follow %s", article.Author.Username)
			}
		}
	})

	t.Run("Current user doesn't follow themselves", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Token " + aliceToken}
		res, err := ts.executeRequest(http.MethodGet, "/articles", "", headers)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)

		var response struct {
			Articles      []data.Article `json:"articles"`
			ArticlesCount int            `json:"articlesCount"`
		}
		readJsonResponse(t, res.Body, &response)

		// Alice follows Bob and David, but not herself
		for _, article := range response.Articles {
			if article.Author.Username == "alice" {
				assert.False(t, article.Author.Following, "User should not follow themselves")
			} else if article.Author.Username == "bob" || article.Author.Username == "david" {
				assert.True(t, article.Author.Following, "Alice follows %s", article.Author.Username)
			} else {
				assert.False(t, article.Author.Following, "Alice doesn't follow %s", article.Author.Username)
			}
		}
	})

	t.Run("Combined filters", func(t *testing.T) {
		testCases := []struct {
			name          string
			token         string
			queryString   string
			expectedCount int
			checkAuthor   string
			checkTag      string
		}{
			{
				name:          "tag and author",
				token:         bobToken,
				queryString:   "/articles?tag=golang&author=alice",
				expectedCount: 3,
				checkAuthor:   "alice",
				checkTag:      "golang",
			},
			{
				name:          "overlapping tag and author",
				token:         emilyToken,
				queryString:   "/articles?tag=backend&author=charlie",
				expectedCount: 2,
				checkAuthor:   "charlie",
				checkTag:      "backend",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				headers := map[string]string{"Authorization": "Token " + tc.token}
				res, err := ts.executeRequest(http.MethodGet, tc.queryString, "", headers)
				require.NoError(t, err)
				defer res.Body.Close()

				require.Equal(t, http.StatusOK, res.StatusCode)

				var response struct {
					Articles      []data.Article `json:"articles"`
					ArticlesCount int            `json:"articlesCount"`
				}
				readJsonResponse(t, res.Body, &response)

				assert.Equal(t, tc.expectedCount, response.ArticlesCount)
				assert.Len(t, response.Articles, tc.expectedCount)

				// Verify all match both filters
				for _, article := range response.Articles {
					assert.Equal(t, tc.checkAuthor, article.Author.Username)
					assert.Contains(t, article.TagList, tc.checkTag)
				}
			})
		}
	})

	t.Run("No results when filter matches nothing", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Token " + aliceToken}
		res, err := ts.executeRequest(http.MethodGet, "/articles?tag=nonexistent", "", headers)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)

		var response struct {
			Articles      []data.Article `json:"articles"`
			ArticlesCount int            `json:"articlesCount"`
		}
		readJsonResponse(t, res.Body, &response)

		assert.Equal(t, 0, response.ArticlesCount)
		assert.Empty(t, response.Articles)
	})

	t.Run("Articles have all required fields", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Token " + aliceToken}
		res, err := ts.executeRequest(http.MethodGet, "/articles?limit=1", "", headers)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)

		var response struct {
			Articles      []data.Article `json:"articles"`
			ArticlesCount int            `json:"articlesCount"`
		}
		readJsonResponse(t, res.Body, &response)
		require.Len(t, response.Articles, 1)

		article := response.Articles[0]
		assert.NotEmpty(t, article.Slug)
		assert.NotEmpty(t, article.Title)
		assert.NotEmpty(t, article.Description)
		assert.Empty(t, article.Body, "Body should not be included in list results")
		assert.NotNil(t, article.TagList)
		assert.NotZero(t, article.CreatedAt)
		assert.NotZero(t, article.UpdatedAt)
		assert.True(t, article.FavoritesCount >= 0)
		assert.NotEmpty(t, article.Author.Username)
	})

	t.Run("Filter validation errors", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Token " + aliceToken}

		testCases := []struct {
			name          string
			queryString   string
			expectedError string
		}{
			{
				name:          "tag with special characters",
				queryString:   "/articles?tag=golang-test",
				expectedError: "Tag must contain only alphanumeric characters",
			},
			{
				name:          "tag too long",
				queryString:   "/articles?tag=" + strings.Repeat("a", 51),
				expectedError: "Tag must not be more than 50 characters",
			},
			{
				name:          "author with special characters",
				queryString:   "/articles?author=alice@test",
				expectedError: "Author must contain only alphanumeric characters",
			},
			{
				name:          "author too long",
				queryString:   "/articles?author=" + strings.Repeat("a", 51),
				expectedError: "Author must not be more than 50 characters",
			},
			{
				name:          "favorited with special characters",
				queryString:   "/articles?favorited=bob_user",
				expectedError: "Favorited username must contain only alphanumeric characters",
			},
			{
				name:          "favorited too long",
				queryString:   "/articles?favorited=" + strings.Repeat("a", 51),
				expectedError: "Favorited username must not be more than 50 characters",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				res, err := ts.executeRequest(http.MethodGet, tc.queryString, "", headers)
				require.NoError(t, err)
				defer res.Body.Close()

				require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)

				var response errorResponse
				readJsonResponse(t, res.Body, &response)
				assert.Contains(t, response.Errors, tc.expectedError)
			})
		}
	})

	t.Run("Multiple validation errors", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Token " + aliceToken}
		longTag := strings.Repeat("a", 51)
		res, err := ts.executeRequest(http.MethodGet, "/articles?tag="+longTag+"&author=test-user&favorited=user@test", "", headers)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)

		var response errorResponse
		readJsonResponse(t, res.Body, &response)
		assert.GreaterOrEqual(t, len(response.Errors), 3, "Should have multiple validation errors")
		assert.Contains(t, response.Errors, "Tag must not be more than 50 characters")
		assert.Contains(t, response.Errors, "Author must contain only alphanumeric characters")
		assert.Contains(t, response.Errors, "Favorited username must contain only alphanumeric characters")
	})

	t.Run("Valid filters pass validation", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Token " + aliceToken}
		res, err := ts.executeRequest(http.MethodGet, "/articles?tag=golang&author=alice&limit=10&offset=0", "", headers)
		require.NoError(t, err)
		defer res.Body.Close()

		assert.Equal(t, http.StatusOK, res.StatusCode)
	})
}

func TestArticleStore_GetIDBySlug(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	// Setup: Register user and create an article
	registerUser(t, ts, "alice", "alice@example.com", "password123")
	aliceToken := loginUser(t, ts, "alice@example.com", "password123")

	// Create an article
	articleLocation := createArticle(t, ts, aliceToken, "Test Article for ID Lookup", "Testing ID by slug", "Article body content", []string{"test"})

	// Extract slug from location header (format: /articles/test-article-for-id-lookup-xxxxxx)
	slug := articleLocation[10:] // Remove "/articles/" prefix

	// Test: Get article ID by slug
	articleID, err := ts.app.modelStore.Articles.GetIDBySlug(slug)
	require.NoError(t, err)
	require.NotZero(t, articleID, "Article ID should not be zero")

	// Verify it's the correct ID by getting the full article
	fullArticle, err := ts.app.modelStore.Articles.GetBySlug(slug, data.AnonymousUser)
	require.NoError(t, err)
	require.Equal(t, fullArticle.ID, articleID, "IDs should match")

	// Test: Non-existent slug
	nonExistentID, err := ts.app.modelStore.Articles.GetIDBySlug("non-existent-slug-12345")
	require.Error(t, err)
	require.Equal(t, data.ErrRecordNotFound, err, "Should return ErrRecordNotFound for non-existent slug")
	require.Zero(t, nonExistentID, "ID should be zero for non-existent article")
}
