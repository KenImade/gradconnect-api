package main

import (
	"net/http"
	"time"

	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/julienschmidt/httprouter"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "api.gradconnect.com/cmd/api/docs"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

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

	// opportunities
	router.HandlerFunc(http.MethodGet, "/api/v1/opportunities", app.listOpportunitiesHandler)
	router.HandlerFunc(http.MethodGet, "/api/v1/opportunities/:slug", app.showOpportunityBySlugHandler)

	// auth (with endpoint-specific rate limits)
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/register",
		app.rateLimitByIP(3, time.Hour)(app.registerUserHandler))
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/google", app.googleAuthHandler)
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/login", app.loginUserHandler)
	router.HandlerFunc(http.MethodGet, "/api/v1/auth/verify-email", app.activateUserHandler)
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/forgot-password", app.forgotPasswordHandler)
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/reset-password", app.resetPasswordHandler)

	// Authenticated routes

	// auth
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/logout",
		app.requireAuthenticatedUser(app.logoutUserHandler))
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/resend-verification",
		app.requireAuthenticatedUser(app.resendVerificationEmailHandler))

	// me
	router.HandlerFunc(http.MethodGet, "/api/v1/me",
		app.requireAuthenticatedUser(app.getCurrentUserHandler))
	router.HandlerFunc(http.MethodPatch, "/api/v1/me",
		app.requireAuthenticatedUser(app.updateUserHandler))

	// bookmark
	router.HandlerFunc(http.MethodGet, "/api/v1/me/bookmarks",
		app.requireVerifiedUser(app.listBookmarksHandler))
	router.HandlerFunc(http.MethodPost, "/api/v1/me/bookmarks",
		app.requireVerifiedUser(app.addBookmarkHandler))
	router.HandlerFunc(http.MethodDelete, "/api/v1/me/bookmarks/:id",
		app.requireVerifiedUser(app.removeBookmarkHandler))

	// application
	router.HandlerFunc(http.MethodGet, "/api/v1/me/applications",
		app.requireVerifiedUser(app.listApplicationsHandler))
	router.HandlerFunc(http.MethodPost, "/api/v1/me/applications",
		app.requireVerifiedUser(app.addApplicationHandler))
	router.HandlerFunc(http.MethodPatch, "/api/v1/me/applications/:id",
		app.requireVerifiedUser(app.updateApplicationHandler))
	router.HandlerFunc(http.MethodDelete, "/api/v1/me/applications/:id",
		app.requireVerifiedUser(app.removeApplicationHandler))

	// review
	router.HandlerFunc(http.MethodPost, "/api/v1/reviews",
		app.requirePermission("review:submit", app.addReviewHandler))
	router.HandlerFunc(http.MethodPatch, "/api/v1/reviews/:id",
		app.requirePermission("review:edit", app.updateReviewHandler))

	// Admin employer routes
	router.HandlerFunc(http.MethodPost, "/api/v1/admin/employers",
		app.requirePermission("admin:full", app.createEmployerHandler))
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/employers/:id",
		app.requirePermission("admin:full", app.showEmployerByIDHandler))
	router.HandlerFunc(http.MethodPatch, "/api/v1/admin/employers/:id",
		app.requirePermission("admin:full", app.updateEmployerHandler))
	router.HandlerFunc(http.MethodDelete, "/api/v1/admin/employers/:id",
		app.requirePermission("admin:full", app.deleteEmployerHandler))

	// Admin opportunities routes
	router.HandlerFunc(http.MethodPost, "/api/v1/admin/opportunities",
		app.requirePermission("admin:full", app.createOpportunityHandler))
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/opportunities/:id",
		app.requirePermission("admin:full", app.showOpportunityByIDHandler))
	router.HandlerFunc(http.MethodPatch, "/api/v1/admin/opportunities/:id",
		app.requirePermission("admin:full", app.updateOpportunityHandler))
	router.HandlerFunc(http.MethodDelete, "/api/v1/admin/opportunities/:id",
		app.requirePermission("admin:full", app.deleteOpportunityHandler))
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/opportunities",
		app.requirePermission("admin:full", app.listAdminOpportunitiesHandler))

	// Admin assessments routes
	router.HandlerFunc(http.MethodPost, "/api/v1/admin/assessments",
		app.requirePermission("admin:full", app.createAssessmentHandler))
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/assessments/:id",
		app.requirePermission("admin:full", app.showAdminAssessmentHandler))
	router.HandlerFunc(http.MethodPatch, "/api/v1/admin/assessments/:id",
		app.requirePermission("admin:full", app.updateAssessmentHandler))
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/assessments",
		app.requirePermission("admin:full", app.listAdminAssessmentsHandler))

	// Admin review routes
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/reviews",
		app.requirePermission("admin:full", app.listReviewsForModerationHandler))
	router.HandlerFunc(http.MethodPatch, "/api/v1/admin/reviews/:id",
		app.requirePermission("admin:full", app.moderateReviewHandler))

	// Admin import routes
	router.HandlerFunc(http.MethodPost, "/api/v1/admin/import",
		app.requirePermission("admin:full", app.startImportHandler))
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/import/:id",
		app.requirePermission("admin:full", app.getImportJobHandler))

	// Admin job triggers
	router.HandlerFunc(http.MethodPost, "/api/v1/admin/jobs/deadline-reminders",
		app.requirePermission("admin:full", app.triggerDeadlineRemindersHandler))

	// Build the middleware chain. Inner-to-outer order:
	//   router → authenticate → rateLimitAll → enableCORS → logRequests → Sentry
	//
	// Sentry is outermost so it sees every request before any other middleware
	// runs, captures panics before they're handled, and tags every Sentry event
	// with the full request context (URL, method, headers).
	sentryMiddleware := sentryhttp.New(sentryhttp.Options{
		Repanic: true, // re-panic after capturing so logRequests can record the 500
	})

	return sentryMiddleware.Handle(
		app.logRequests(
			app.enableCORS(
				app.rateLimitAll()(
					app.authenticate(router),
				),
			),
		),
	)
}
