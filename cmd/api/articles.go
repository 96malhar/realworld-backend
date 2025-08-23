package main

import (
	"net/http"

	"github.com/96malhar/realworld-backend/internal/data"
	"github.com/96malhar/realworld-backend/internal/validator"
)

func (app *application) createArticleHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Article struct {
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Body        string   `json:"body"`
			TagList     []string `json:"tagList"`
		} `json:"article"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	article := &data.Article{
		Title:       input.Article.Title,
		Description: input.Article.Description,
		Body:        input.Article.Body,
		TagList:     input.Article.TagList,
		AuthorID:    app.contextGetUser(r).ID,
	}

	v := validator.New()

	if data.ValidateArticle(v, article); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.modelStore.Articles.Insert(article)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// set location header to point to the new article
	headers := make(http.Header)
	headers.Set("Location", "/api/articles/"+article.Slug)
	err = app.writeJSON(w, http.StatusCreated, envelope{}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}
