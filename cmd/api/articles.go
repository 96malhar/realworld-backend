package main

import (
	"errors"
	"net/http"

	"github.com/96malhar/realworld-backend/internal/data"
	"github.com/96malhar/realworld-backend/internal/validator"
	"github.com/go-chi/chi/v5"
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
	headers.Set("Location", "/articles/"+article.Slug)
	err = app.writeJSON(w, http.StatusCreated, envelope{}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

func (app *application) getArticleHandler(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	article, err := app.modelStore.Articles.GetBySlug(slug, app.contextGetUser(r))
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFoundResponse(w, r)
			return
		}
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"article": article}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

func (app *application) favoriteArticleHandler(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	user := app.contextGetUser(r)

	article, err := app.modelStore.Articles.FavoriteBySlug(slug, user.ID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFoundResponse(w, r)
			return
		}
		app.serverErrorResponse(w, r, err)
		return
	}

	if err := app.writeJSON(w, http.StatusOK, envelope{"article": article}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

func (app *application) unfavoriteArticleHandler(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	user := app.contextGetUser(r)

	article, err := app.modelStore.Articles.UnfavoriteBySlug(slug, user.ID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFoundResponse(w, r)
			return
		}
		app.serverErrorResponse(w, r, err)
		return
	}

	if err := app.writeJSON(w, http.StatusOK, envelope{"article": article}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}
