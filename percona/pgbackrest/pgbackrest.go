package pgbackrest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"github.com/superfly/percona-postgresql-operator/internal/feature"
	"github.com/superfly/percona-postgresql-operator/internal/naming"
	pgbackrestapi "github.com/superfly/percona-postgresql-operator/internal/pgbackrest-server/api"
	"github.com/superfly/percona-postgresql-operator/percona/clientcmd"
	v2 "github.com/superfly/percona-postgresql-operator/pkg/apis/pgv2.percona.com/v2"
)

type InfoOutput []InfoStanza

type InfoBackup struct {
	Annotation map[string]string `json:"annotation,omitempty"`
	Label      string            `json:"label,omitempty"`
	Type       v2.PGBackupType   `json:"type,omitempty"`
	Timestamp  struct {
		Start int64 `json:"start,omitempty"`
		Stop  int64 `json:"stop,omitempty"`
	} `json:"timestamp,omitempty"`
}

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

var ErrNoValidBackups = errors.New("no valid backups")

const (
	statusOK = 0
	// statusNoValidBackups means that there are no backups in pgbackrest
	statusNoValidBackups = 2
)

func GetInfo(ctx context.Context, pod *corev1.Pod, repoName string) (InfoOutput, error) {
	out := InfoOutput{}

	if feature.Enabled(ctx, feature.PGBackRestHTTP) {
		repo, err := strconv.Atoi(strings.TrimPrefix(repoName, "repo"))
		if err != nil {
			return InfoOutput{}, errors.Wrap(err, "failed to parse repo number")
		}

		opts := pgbackrestapi.InfoCommandOptions{
			Output: "json",
			Repo:   repo,
		}

		var body bytes.Buffer
		err = json.NewEncoder(&body).Encode(opts)
		if err != nil {
			return InfoOutput{}, errors.Wrap(err, "failed to marshal pgBackRest info options")
		}

		url := fmt.Sprintf("https://%s.%s-pods.%s.svc:4422/pgbackrest/api", pod.Name, pod.Namespace, pod.Namespace)
		req, err := http.NewRequest(http.MethodPost, url, &body)
		if err != nil {
			return InfoOutput{}, errors.Wrap(err, "failed to create request")
		}
		req.Header.Set("Content-Type", "application/json")
		// req.Header.Set("User-Agent", "")

		client := &http.Client{
			Transport: cleanhttp.DefaultPooledTransport(),
		}
		resp, err := client.Do(req)

		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return InfoOutput{}, errors.Wrap(err, "failed to decode pgBackRest info response")
		}
	} else {
		stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)

		c, err := clientcmd.NewClient()
		if err != nil {
			return InfoOutput{}, errors.Wrap(err, "failed to create client")
		}

		if err := c.Exec(ctx, pod, naming.ContainerDatabase, nil, stdout, stderr, "pgbackrest", "info", "--output=json", "--repo="+strings.TrimPrefix(repoName, "repo")); err != nil {
			return InfoOutput{}, errors.Wrapf(err, "exec: %s", stderr.String())
		}

		// if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		// 	return InfoOutput{}, errors.Wrap(err, "failed to unmarshal pgBackRest info output")
		// }
		if err := json.NewDecoder(stdout).Decode(&out); err != nil {
			return InfoOutput{}, errors.Wrap(err, "failed to unmarshal pgBackRest info output")
		}
	}

	for _, elem := range out {
		switch elem.Status.Code {
		case statusNoValidBackups:
			return InfoOutput{}, ErrNoValidBackups
		case statusOK:
			continue
		default:
			return InfoOutput{}, errors.Errorf("pgBackRest info command failed with code %d: %s", int(elem.Status.Code), elem.Status.Message)
		}
	}

	return out, nil
}

func SetAnnotationsToBackup(ctx context.Context, pod *corev1.Pod, stanza string, backupSet string, repoName string, annotations map[string]string) error {
	stderr := new(bytes.Buffer)

	c, err := clientcmd.NewClient()
	if err != nil {
		return errors.Wrap(err, "failed to create client")
	}

	annotationsOpts := []string{}
	for k, v := range annotations {
		annotationsOpts = append(annotationsOpts, fmt.Sprintf(`--annotation=%s=%s`, k, v))
	}

	cmd := []string{"pgbackrest", fmt.Sprintf(`--stanza=%s`, stanza), "--set=" + backupSet, "--repo=" + strings.TrimPrefix(repoName, "repo")}
	cmd = append(cmd, annotationsOpts...)
	cmd = append(cmd, "annotate")

	if err := c.Exec(ctx, pod, naming.ContainerDatabase, nil, nil, stderr, cmd...); err != nil {
		return errors.Wrapf(err, "exec: %s", stderr.String())
	}

	return nil
}
