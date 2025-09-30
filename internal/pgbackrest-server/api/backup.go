package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	pgbackrestserver "github.com/superfly/percona-postgresql-operator/internal/pgbackrest-server"
)

type BackupCommandOptions struct {
	// General Options
	Stanza       string `json:"stanza"`         // Required
	Type         string `json:"type,omitempty"` // 'full', 'incr', or 'diff'
	LockPath     string `json:"lock_path,omitempty"`
	ProcessMax   int    `json:"process_max,omitempty"`
	CompressType string `json:"compress_type,omitempty"`

	// Backup Options
	Annotations   map[string]string `json:"annotations,omitempty"`
	Force         bool              `json:"force,omitempty"`
	Resume        bool              `json:"resume,omitempty"`
	StartFast     bool              `json:"start_fast,omitempty"`
	StopAuto      bool              `json:"stop_auto,omitempty"`
	BackupStandby bool              `json:"backup_standby,omitempty"`

	// Filter Options
	DBInclude []string `json:"db_include,omitempty"`

	// Repository Options
	Repo                  int    `json:"repo,omitempty"`
	RepoPath              string `json:"repo_path,omitempty"`
	RepoType              string `json:"repo_type,omitempty"`
	RepoHardlink          bool   `json:"repo_hardlink,omitempty"`
	RepoRetentionFull     int    `json:"repo_retention_full,omitempty"`
	RepoRetentionFullType string `json:"repo_retention_full_type,omitempty"`

	// Logging and Configuration
	Config string `json:"config,omitempty"`
}

func (c *BackupCommandOptions) AsSlice() []string {
	return pgbackrestserver.IntoOpts(reflect.ValueOf(c))
}

func BackupCommandHandler(w http.ResponseWriter, r *http.Request) {
	var opts BackupCommandOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		http.Error(w, fmt.Sprintf("could not decode json body: %v", err), http.StatusBadRequest)
		return
	}

	cmd := pgbackrestserver.BackrestCommand{
		Command: "backup",
		Opts:    opts.AsSlice(),
	}

	stdout, _, err := cmd.Run(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to run pgbackrest backup command: %v", err), http.StatusInternalServerError)
		return
	}

	out := map[string]string{"data": stdout.String()}
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode json response: %v", err), http.StatusInternalServerError)
		return
	}

}
