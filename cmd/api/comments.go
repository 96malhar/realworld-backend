package main

import (
	"errors"
	"net/http"

	"github.com/96malhar/realworld-backend/internal/data"
	"github.com/96malhar/realworld-backend/internal/validator"
	"github.com/go-chi/chi/v5"
)

func (app *application) createCommentHandler(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var input struct {
		Comment struct {
			Body string `json:"body"`
		} `json:"comment"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Get the article ID by slug
	articleID, err := app.modelStore.Articles.GetIDBySlug(slug)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFoundResponse(w, r)
			return
		}
		app.serverErrorResponse(w, r, err)
		return
	}

	comment := &data.Comment{
		Body:      input.Comment.Body,
		ArticleID: articleID,
		AuthorID:  app.contextGetUser(r).ID,
	}

	v := validator.New()

	if data.ValidateComment(v, comment); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.modelStore.Comments.Insert(comment)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	currentUser := app.contextGetUser(r)

	// Build the comment author profile (user doesn't follow themselves)
	comment.Author = currentUser.ToProfile(false)

	err = app.writeJSON(w, http.StatusCreated, envelope{"comment": comment}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

func (app *application) getCommentsHandler(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	// Get the article ID by slug (verifies article exists)
	articleID, err := app.modelStore.Articles.GetIDBySlug(slug)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFoundResponse(w, r)
			return
		}
		app.serverErrorResponse(w, r, err)
		return
	}

	// Get all comments for the article (includes author details via JOIN)
	comments, err := app.modelStore.Comments.GetByArticleID(articleID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Set following status if user is authenticated (single bulk query)
	currentUser := app.contextGetUser(r)
	if !currentUser.IsAnonymous() {
		err = app.modelStore.Comments.SetFollowingStatus(comments, currentUser.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"comments": comments}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}
