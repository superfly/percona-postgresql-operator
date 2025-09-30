package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	pgbackrestserver "github.com/superfly/percona-postgresql-operator/internal/pgbackrest-server"
)

type InfoCommandOptions struct {
	// General Options
	Stanza            string `json:"stanza"` // Required
	Config            string `json:"config,omitempty"`
	ConfigIncludePath string `json:"config_include_path,omitempty"`
	ConfigPath        string `json:"config_path,omitempty"`
	Output            string `json:"output,omitempty"`

	// Backup Set & Filter Options
	BackupSet string   `json:"set,omitempty"`
	DBInclude []string `json:"db_include,omitempty"`
	Type      string   `json:"type,omitempty"`

	// Archive Options
	ArchiveCheck bool `json:"archive_check,omitempty"`
	ArchiveCopy  bool `json:"archive_copy,omitempty"`

	// Repository Options
	Repo                  int    `json:"repo,omitempty"`
	RepoPath              string `json:"repo_path,omitempty"`
	RepoType              string `json:"repo_type,omitempty"`
	RepoHardlink          bool   `json:"repo_hardlink,omitempty"`
	RepoRetentionFull     int    `json:"repo_retention_full,omitempty"`
	RepoRetentionFullType string `json:"repo_retention_full_type,omitempty"`

	// S3 Repository Options
	RepoS3Bucket   string `json:"repo_s3_bucket,omitempty"`
	RepoS3Endpoint string `json:"repo_s3_endpoint,omitempty"`
	RepoS3Region   string `json:"repo_s3_region,omitempty"`
}

func (c *InfoCommandOptions) AsSlice() []string {
	return pgbackrestserver.IntoOpts(reflect.ValueOf(c))
}

func InfoCommandHandler(w http.ResponseWriter, r *http.Request) {
	var opts InfoCommandOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		http.Error(w, fmt.Sprintf("could not decode json body: %v", err), http.StatusBadRequest)
		return
	}

	cmd := pgbackrestserver.BackrestCommand{
		Command: "info",
		Opts:    opts.AsSlice(),
	}

	stdout, _, err := cmd.Run(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to run pgbackrest info command: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(stdout.Bytes())
}
