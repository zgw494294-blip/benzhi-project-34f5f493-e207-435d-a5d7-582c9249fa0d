package web

import (
	"net/http"
	"powerpermit/internal/application"
	"powerpermit/internal/domain"
	"strings"
)

func (s *Server) HomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := assets.ReadFile("assets/index.html")
	if err != nil {
		http.Error(w, "页面资源不可用", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if err := s.service.AssertHealthy(); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ListCasesHandler(w http.ResponseWriter, r *http.Request) {
	status := domain.CaseStatus(r.URL.Query().Get("status"))
	if status != "" && !validStatus(status) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "未知案件状态", Code: "VALIDATION", Field: "status"})
		return
	}
	items, err := s.service.Search(strings.TrimSpace(r.URL.Query().Get("q")), status)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

func validStatus(status domain.CaseStatus) bool {
	switch status {
	case domain.StatusDraft, domain.StatusPlanned, domain.StatusInspecting, domain.StatusRectifying, domain.StatusReviewing, domain.StatusFrozen, domain.StatusPermitted:
		return true
	default:
		return false
	}
}

func (s *Server) CreateCaseHandler(w http.ResponseWriter, r *http.Request) {
	var command application.CreateCaseCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	response, err := s.service.CreateCase(command)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusCreated
	if response.Duplicate {
		status = http.StatusOK
	}
	writeJSON(w, status, response)
}

func (s *Server) GetCaseHandler(w http.ResponseWriter, r *http.Request) {
	aggregate, err := s.service.Get(r.PathValue("caseID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, aggregate)
}
