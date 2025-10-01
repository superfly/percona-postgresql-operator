package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	pgbackrestserver "github.com/superfly/percona-postgresql-operator/internal/pgbackrest-server"
)

type RestoreCommandOptions struct {
	// General Options
	Stanza string `json:"stanza"` // Required
	Set    string `json:"set,omitempty"`
	Delta  bool   `json:"delta,omitempty"`
	Force  bool   `json:"force,omitempty"`
	Repo   int    `json:"repo,omitempty"`

	// Filter Options
	DBInclude []string `json:"db_include,omitempty"`

	// Target & Recovery Options (Point-in-Time Recovery)
	TargetType     string            `json:"target_type,omitempty"` // e.g., 'time', 'xid', 'lsn'
	Target         string            `json:"target,omitempty"`
	TargetAction   string            `json:"target_action,omitempty"` // e.g., 'pause', 'promote', 'shutdown'
	RecoveryOption map[string]string `json:"recovery_option,omitempty"`
}

func (c *RestoreCommandOptions) AsSlice() []string {
	return pgbackrestserver.IntoOpts(reflect.ValueOf(c))
}

func RestoreCommandHandler(w http.ResponseWriter, r *http.Request) {
	var opts RestoreCommandOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		http.Error(w, fmt.Sprintf("could not decode json body: %v", err), http.StatusBadRequest)
		return
	}

	cmd := pgbackrestserver.BackrestCommand{
		Command: "restore",
		Opts:    opts.AsSlice(),
	}

	stdout, _, err := cmd.Run(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to run pgbackrest restore command: %v", err), http.StatusInternalServerError)
		return
	}

	out := map[string]string{"data": stdout.String()}
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode json response: %v", err), http.StatusInternalServerError)
		return
	}

}
