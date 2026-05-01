package main

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const maxLogoSize = 2 << 20 // 2MB
var allowedLogoExtensions = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
}

// uploadLogoHandler godoc
// @Summary      Upload an employer logo (admin only)
// @Description  Uploads an image to object storage and returns its public URL.
// @Description  Accepts PNG, JPEG, WebP, and SVG up to 2MB. Requires admin:full permission.
// @Tags         Admin
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "Image file (PNG, JPEG, WebP, SVG)"
// @Success      201   {object}  envelope{data=object{url=string}}
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      413   {object}  ErrorResponse
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /admin/uploads/logo [post]
func (app *application) uploadLogoHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLogoSize)

	if err := r.ParseMultipartForm(maxLogoSize); err != nil {
		app.errorResponse(w, r, http.StatusRequestEntityTooLarge, "file too large (max 2MB)")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		app.badRequestResponse(w, r, errors.New("missing 'file' field in form data"))
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentType, ok := allowedLogoExtensions[ext]
	if !ok {
		app.errorResponse(w, r, http.StatusUnprocessableEntity,
			"unsupported file type; allowed: PNG, JPEG, WebP, SVG")
		return
	}

	// logos/<uuid>.<ext> — UUID prevents collisions and ensures cache busting
	// when an employer's logo is re-uploaded.
	storageKey := fmt.Sprintf("logos/%s%s", uuid.NewString(), ext)

	publicURL, err := app.storage.Upload(r.Context(), storageKey, contentType, file)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{
		"data": map[string]any{
			"url": publicURL,
			"key": storageKey,
		},
	}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
