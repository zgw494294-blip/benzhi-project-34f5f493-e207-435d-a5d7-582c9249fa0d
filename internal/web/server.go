package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"powerpermit/internal/application"
)

//go:embed assets/*
var assets embed.FS

type Server struct {
	service *application.Service
	mux     *http.ServeMux
}

func New(service *application.Service) *Server {
	s := &Server{service: service, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return securityHeaders(s.mux) }

func (s *Server) routes() {
	assetFS, _ := fs.Sub(assets, "assets")
	s.mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetFS))))
	s.mux.HandleFunc("GET /", s.HomeHandler)
	s.mux.HandleFunc("GET /permits/{caseID}/print", s.PermitPrintHandler)
	s.mux.HandleFunc("GET /healthz", s.HealthHandler)
	s.mux.HandleFunc("GET /api/cases", s.ListCasesHandler)
	s.mux.HandleFunc("GET /api/dashboard", s.DashboardHandler)
	s.mux.HandleFunc("GET /api/checklist", s.ChecklistHandler)
	s.mux.HandleFunc("GET /api/permits/{permitNumber}", s.FindPermitHandler)
	s.mux.HandleFunc("POST /api/cases", s.CreateCaseHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}", s.GetCaseHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/plans", s.SubmitPlanHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/inspections/start", s.StartInspectionHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/inspections", s.RecordInspectionHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/findings/{findingID}/evidence", s.SubmitEvidenceHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/review", s.ReviewHandler)
	s.mux.HandleFunc("POST /api/cases/{caseID}/permit", s.IssuePermitHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}/permit", s.GetPermitHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}/audit", s.AuditHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}/readiness", s.ReadinessHandler)
	s.mux.HandleFunc("GET /api/cases/{caseID}/history", s.HistoryHandler)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			writeJSON(w, http.StatusUnsupportedMediaType, errorResponse{Error: "POST 请求必须使用 application/json", Code: "CONTENT_TYPE"})
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
