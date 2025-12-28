package handlers

import (
	"net/http"
	"strings"

	"pricesapi/internal/config"
	"pricesapi/internal/prices"
)

func PostPrices(svc *prices.Service, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		archType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
		if archType == "" {
			archType = "zip"
		}
		if archType != "zip" && archType != "tar" {
			badRequest(w, "query param 'type' must be 'zip' or 'tar'")
			return
		}

		tempPath, cleanup, err := prices.ExtractUploadToTempFile(r, cfg.MaxUploadMB)
		if err != nil {
			badRequest(w, err.Error())
			return
		}
		defer cleanup()

		res, err := svc.ImportArchive(r.Context(), tempPath, archType)
		if err != nil {
			serverError(w, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, res)
	}
}
