package app

import "net/http"

// @Summary      Healthcheck
// @Description  Returns API status and environment info
// @Tags         System
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /healthcheck [get]
func (app *App) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	env := envelope{
		"status": "available",
		"system_info": map[string]any{
			"environment": app.config.Env,
			"version":     Version,
		},
	}

	err := app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
