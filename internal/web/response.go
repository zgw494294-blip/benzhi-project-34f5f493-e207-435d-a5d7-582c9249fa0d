package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"powerpermit/internal/domain"
)

type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
	Field string `json:"field,omitempty"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "请求 JSON 无效：" + err.Error(), Code: "BAD_JSON"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "请求只能包含一个 JSON 对象", Code: "BAD_JSON"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "INTERNAL"
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status, code = http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, domain.ErrConflict):
		status, code = http.StatusConflict, "VERSION_CONFLICT"
	case errors.Is(err, domain.ErrInvalidState):
		status, code = http.StatusConflict, "INVALID_STATE"
	case errors.Is(err, domain.ErrForbidden):
		status, code = http.StatusForbidden, "FORBIDDEN"
	}
	response := errorResponse{Error: err.Error(), Code: code}
	var validation *domain.ValidationError
	if errors.As(err, &validation) {
		status, response.Code, response.Field = http.StatusUnprocessableEntity, "VALIDATION", validation.Field
	}
	writeJSON(w, status, response)
}
