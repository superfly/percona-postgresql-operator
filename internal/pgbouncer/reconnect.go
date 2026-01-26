package pgbouncer

import (
	"bytes"
	"context"

	"github.com/percona/percona-postgresql-operator/internal/logging"
	"github.com/percona/percona-postgresql-operator/internal/postgres"
)

// Reconnect sends the RECONNECT command to PgBouncer's admin console.
// This closes all server connections at the next opportunity, forcing
// PgBouncer to establish new connections to the current primary.
//
// From PgBouncer docs: "Close each open server connection for the given
// database, or all databases, at the next opportunity."
//
// This is non-disruptive: PgBouncer waits for the connection to be
// "released" before closing it. In transaction pooling mode, this means
// waiting for the current transaction to complete. In session pooling
// mode, this means waiting for the client to disconnect.
//
// The command connects via Unix socket as the special "pgbouncer" user,
// which is allowed without password when the client has the same UID
// as the running PgBouncer process.
//
// Ref.: https://www.pgbouncer.org/usage.html#admin-console
func Reconnect(ctx context.Context, exec postgres.Executor) error {
	log := logging.FromContext(ctx)
	log.Info("Triggering PgBouncer RECONNECT to force new server connections")

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}

	// Connect to PgBouncer admin console via Unix socket.
	// The "pgbouncer" user can connect without password from same UID.
	// The "pgbouncer" database is the virtual admin console.
	// Ref.:
	err := exec(ctx, nil, stdout, stderr,
		"psql",
		"-h", "/tmp/pgbouncer",
		"-p", "6432",
		"-U", "pgbouncer",
		"-d", "pgbouncer",
		"-c", "RECONNECT",
	)

	if err != nil {
		log.Error(err, "RECONNECT failed",
			"stdout", stdout.String(),
			"stderr", stderr.String())
	} else {
		log.V(1).Info("RECONNECT succeeded",
			"stdout", stdout.String())
	}

	return err
}
