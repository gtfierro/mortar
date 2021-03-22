package server

import (
	"context"
	"net/http"

	"github.com/gtfierro/mortar2/internal/database"
	"github.com/gtfierro/mortar2/internal/logging"
)

func addLogger(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(logging.WithLogger(r.Context()))
		next(w, r)
	})
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apikey := r.URL.Query().Get("apikey")
		if len(apikey) == 0 {
			http.Error(w, "Non-existent or invalid apikey", http.StatusUnauthorized)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), database.ContextKey("user"), apikey))
		next(w, r)
	})
}
