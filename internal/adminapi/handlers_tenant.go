package adminapi

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5"
)

type OIDCBody struct {
	OAuthIssuer       string   `json:"oauth_issuer"`
	AcceptedAudiences []string `json:"accepted_audiences"`
	ClientIDUser      string   `json:"client_id_user"`
	ClientIDMachine   *string  `json:"client_id_machine"`
	AccountClaim      string   `json:"account_claim"`
	DpopRequired      *bool    `json:"dpop_required"`
}

func (a *App) getTenantSelf(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	var row struct {
		ID, Slug, Host, Issuer, AccountClaim string
		AcceptedAudiences                    []string
		ClientIDUser, ClientIDMachine        sql.NullString
		DpopRequired                         bool
	}
	err := a.db.QueryRow(r.Context(), `
        SELECT id, slug, host, oauth_issuer, account_claim, accepted_audiences, client_id_user, client_id_machine, dpop_required
        FROM tenants WHERE id = $1
    `, tid).Scan(
		&row.ID, &row.Slug, &row.Host, &row.Issuer, &row.AccountClaim, &row.AcceptedAudiences,
		&row.ClientIDUser, &row.ClientIDMachine, &row.DpopRequired,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "tenant not found", http.StatusNotFound)
			return
		}
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	resp := map[string]any{
		"id":                 row.ID,
		"slug":               row.Slug,
		"host":               row.Host,
		"oauth_issuer":       row.Issuer,
		"accepted_audiences": row.AcceptedAudiences,
		"account_claim":      row.AccountClaim,
		"client_id_user":     nilIfNull(row.ClientIDUser),
		"client_id_machine":  nilIfNull(row.ClientIDMachine),
		"dpop_required":      row.DpopRequired,
	}
	writeJSON(w, resp, 200)
}

func (a *App) putTenantOIDC(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	var b OIDCBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if b.OAuthIssuer == "" || len(b.AcceptedAudiences) == 0 || b.ClientIDUser == "" {
		http.Error(w, "missing required fields", 400)
		return
	}
	_, err := a.db.Exec(r.Context(), `
        UPDATE tenants
        SET oauth_issuer=$1, accepted_audiences=$2, client_id_user=$3, client_id_machine=$4, account_claim=COALESCE($5,'sub'), dpop_required=COALESCE($6,false)
        WHERE id=$7
    `, b.OAuthIssuer, b.AcceptedAudiences, b.ClientIDUser, b.ClientIDMachine, b.AccountClaim, b.DpopRequired, tid)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true}, 200)
}
