package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/superfly/percona-postgresql-operator/cmd/pgbackrest-server/exec"
)

type InfoCommandOutput []InfoStanza

type InfoStanza struct {
	Name   string       `json:"name,omitempty"`
	Backup []InfoBackup `json:"backup,omitempty"`
	Status struct {
		Message string  `json:"message,omitempty"`
		Code    float64 `json:"code,omitempty"`
		Lock    struct {
			Backup struct {
				Held bool `json:"held,omitempty"`
			} `json:"backup,omitempty"`
		} `json:"lock,omitempty"`
	} `json:"status,omitempty"`
}

type InfoBackup struct {
	Annotation map[string]string `json:"annotation,omitempty"`
	Label      string            `json:"label,omitempty"`
	Type       PGBackupType      `json:"type,omitempty"`
	Timestamp  struct {
		Start int64 `json:"start,omitempty"`
		Stop  int64 `json:"stop,omitempty"`
	} `json:"timestamp,omitempty"`
}

type PGBackupType string

const (
	PGBackupTypeFull         PGBackupType = "full"
	PGBackupTypeDifferential PGBackupType = "differential"
	PGBackupTypeIncremental  PGBackupType = "incremental"
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

func (c InfoCommandOptions) AsSlice() []string {
	var cmd []string

	val := reflect.ValueOf(c)
	typ := val.Type()

	for i := range val.NumField() {
		jsonTag := typ.Field(i).Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		field := val.Field(i)
		parts := strings.Split(jsonTag, ",")
		jsonKey := parts[0]
		isOmitEmpty := len(parts) > 1 && parts[1] == "omitempty"

		if isOmitEmpty && field.IsZero() {
			continue
		}

		flag := "--" + strings.ReplaceAll(jsonKey, "_", "-")

		switch field.Kind() {
		case reflect.String:
			if strVal := field.String(); strVal != "" || !isOmitEmpty {
				cmd = append(cmd, fmt.Sprintf(" %s=%s", flag, strVal))
			}
		case reflect.Int, reflect.Int64:
			if intVal := field.Int(); intVal != 0 || !isOmitEmpty {
				cmd = append(cmd, fmt.Sprintf(" %s=%d", flag, intVal))
			}
		case reflect.Bool:
			if field.Bool() {
				cmd = append(cmd, fmt.Sprintf(" %s", flag))
			}
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.String {
				for j := range field.Len() {
					cmd = append(cmd, fmt.Sprintf(" %s=%s", flag, field.Index(j).String()))
				}
			}
		}
	}

	return cmd
}

func InfoCommandHandler(w http.ResponseWriter, r *http.Request) {
	var opts InfoCommandOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		http.Error(w, fmt.Sprintf("could not decode json body: %v", err), http.StatusBadRequest)
		return
	}

	cmd := exec.BackrestCommand{
		Command: "info",
		Opts:    opts.AsSlice(),
	}

	stdout, _, err := cmd.Run(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to run pgbackrest info command: %v", err), http.StatusInternalServerError)
		return
	}

	// var output InfoCommandOutput
	// if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
	// 	// return InfoCommandOutput{}, errors.Wrap(err, "failed to unmarshal pgBackRest info output")
	// 	http.Error(w, fmt.Sprintf("could not decode output from pgbackrest info command: %v", err), http.StatusInternalServerError)
	// 	return
	// }

	w.WriteHeader(http.StatusOK)
	w.Write(stdout.Bytes())
}
