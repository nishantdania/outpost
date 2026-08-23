package httpapi

import "net/http"

func listArks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []string{})
}
