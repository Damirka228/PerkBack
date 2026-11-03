package router

import (
	"net/http"

	health "github.com/Damirka228/PerkBack/services/benefit-service/internal/handlers/health"
	"github.com/go-chi/chi/v5"
)

type Router struct {
	mux *chi.Mux
}

func NewRouter() *Router {
	router := &Router{
		mux: chi.NewRouter(),
	}
	router.registerRoutes()
	return router
}

func (r *Router) registerRoutes() {
	r.mux.Get("/health", health.Check)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}
