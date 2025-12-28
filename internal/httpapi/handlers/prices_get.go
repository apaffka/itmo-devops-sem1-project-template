package handlers

import (
	"net/http"

	"pricesapi/internal/prices"
)

func GetPrices(svc *prices.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := prices.ParseExportFilters(r.URL.Query())
		if err != nil {
			badRequest(w, err.Error())
			return
		}

		zipBytes, err := svc.ExportZip(r.Context(), f)
		if err != nil {
			serverError(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="data.zip"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(zipBytes)
	}
}
