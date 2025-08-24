package adminapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
)

func nilIfNull(s sql.NullString) any {
	if s.Valid {
		return s.String
	}
	return nil
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func writeJSON(w http.ResponseWriter, v any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
