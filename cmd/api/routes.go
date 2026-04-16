package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "api.gradconnect.com/cmd/api/docs"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	// Swagger and Redoc Documentation
	router.HandlerFunc(http.MethodGet, "/", app.redocHandler)
	router.HandlerFunc(http.MethodGet, "/api/v1/docs/redoc", app.redocHandler)
	if app.config.env != "production" {
		router.Handler(http.MethodGet, "/api/v1/docs/swagger/*any", httpSwagger.WrapHandler)
	}

	// health check route
	router.HandlerFunc(http.MethodGet, "/api/v1/healthcheck", app.healthcheckHandler)

	// employer routes
	router.HandlerFunc(http.MethodGet, "/api/v1/employers", app.listEmployersHandler)
	router.HandlerFunc(http.MethodGet, "/api/v1/employers/:slug", app.showEmployerBySlugHandler)

	// assessment routes
	router.HandlerFunc(http.MethodGet, "/api/v1/employers/:slug/assessments", app.listAssessmentsHandler)

	// review routes
	router.HandlerFunc(http.MethodGet, "/api/v1/employers/:slug/reviews", app.listReviewsHandler)

	// Admin
	// router.HandlerFunc(http.MethodGet, "/api/v1/admin/employers/:id", app.sho)

	return router
}
