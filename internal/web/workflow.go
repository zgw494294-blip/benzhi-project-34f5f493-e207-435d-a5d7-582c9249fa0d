package web

import (
	"html/template"
	"net/http"
	"powerpermit/internal/application"
	"powerpermit/internal/domain"
)

func (s *Server) SubmitPlanHandler(w http.ResponseWriter, r *http.Request) {
	var command application.SubmitPlanCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	response, err := s.service.SubmitPlan(r.PathValue("caseID"), command)
	respondCommand(w, response, err)
}

func (s *Server) StartInspectionHandler(w http.ResponseWriter, r *http.Request) {
	var command application.StartInspectionCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	response, err := s.service.StartInspection(r.PathValue("caseID"), command)
	respondCommand(w, response, err)
}

func (s *Server) RecordInspectionHandler(w http.ResponseWriter, r *http.Request) {
	var command application.RecordInspectionCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	response, err := s.service.RecordInspection(r.PathValue("caseID"), command)
	respondCommand(w, response, err)
}

func (s *Server) SubmitEvidenceHandler(w http.ResponseWriter, r *http.Request) {
	var command application.EvidenceCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	response, err := s.service.SubmitEvidence(r.PathValue("caseID"), r.PathValue("findingID"), command)
	respondCommand(w, response, err)
}

func (s *Server) ReviewHandler(w http.ResponseWriter, r *http.Request) {
	var command application.ReviewCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	response, err := s.service.Review(r.PathValue("caseID"), command)
	respondCommand(w, response, err)
}

func (s *Server) IssuePermitHandler(w http.ResponseWriter, r *http.Request) {
	var command application.IssueCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	response, err := s.service.Issue(r.PathValue("caseID"), command)
	respondCommand(w, response, err)
}

func (s *Server) GetPermitHandler(w http.ResponseWriter, r *http.Request) {
	aggregate, err := s.service.Get(r.PathValue("caseID"))
	if err != nil {
		writeError(w, err)
		return
	}
	if aggregate.Permit == nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "送电许可尚未签发", Code: "NOT_FOUND"})
		return
	}
	verified, err := s.service.VerifyPermit(aggregate.Case.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"permit": aggregate.Permit, "verified": verified})
}

func (s *Server) AuditHandler(w http.ResponseWriter, r *http.Request) {
	events, digest, err := s.service.PersistentTimeline(r.PathValue("caseID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": events, "digest": digest})
}

func (s *Server) DashboardHandler(w http.ResponseWriter, r *http.Request) {
	dashboard, err := s.service.Dashboard()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dashboard)
}

func (s *Server) ChecklistHandler(w http.ResponseWriter, r *http.Request) {
	type item struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	items := make([]item, 0, len(domain.Checklist))
	for code, name := range domain.Checklist {
		items = append(items, item{Code: code, Name: name})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	readiness, err := s.service.Readiness(r.PathValue("caseID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, readiness)
}

func (s *Server) HistoryHandler(w http.ResponseWriter, r *http.Request) {
	history, err := s.service.History(r.PathValue("caseID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func (s *Server) FindPermitHandler(w http.ResponseWriter, r *http.Request) {
	aggregate, verified, err := s.service.FindPermit(r.PathValue("permitNumber"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"permit": aggregate.Permit, "case": aggregate.Case, "verified": verified})
}

func respondCommand(w http.ResponseWriter, response application.CommandResponse, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) PermitPrintHandler(w http.ResponseWriter, r *http.Request) {
	aggregate, err := s.service.Get(r.PathValue("caseID"))
	if err != nil || aggregate.Permit == nil {
		http.NotFound(w, r)
		return
	}
	verified, _ := s.service.VerifyPermit(aggregate.Case.ID)
	page := template.Must(template.New("permit").Parse(permitTemplate))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Execute(w, map[string]any{"Aggregate": aggregate, "Verified": verified})
}

const permitTemplate = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>送电许可</title><link rel="stylesheet" href="/assets/style.css"></head><body class="print-page"><main class="permit-sheet"><p class="eyebrow">临时活动用电</p><h1>送电许可</h1><div class="permit-number">{{.Aggregate.Permit.PermitNumber}}</div><dl><dt>活动</dt><dd>{{.Aggregate.Case.ActivityName}}</dd><dt>场地</dt><dd>{{.Aggregate.Case.Venue}}</dd><dt>有效期至</dt><dd>{{.Aggregate.Permit.ValidUntil.Format "2006-01-02 15:04"}}</dd><dt>签发人</dt><dd>{{.Aggregate.Permit.ApprovedBy}}</dd><dt>方案版本</dt><dd>{{.Aggregate.Case.CurrentPlanID}}</dd></dl><p class="hash">校验摘要：{{.Aggregate.Permit.ContentHash}}</p><p class="verified">完整性校验：{{if .Verified}}通过{{else}}失败{{end}}</p><button class="print-button" onclick="window.print()">打印许可</button></main></body></html>`
