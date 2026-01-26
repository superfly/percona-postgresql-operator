package pgbouncer

import (
	"context"

	"github.com/percona/percona-postgresql-operator/internal/logging"
	"github.com/percona/percona-postgresql-operator/internal/postgres"
)

// SignalFailover sends a SIGTERM to PgBouncer to trigger SHUTDOWN WAIT_FOR_CLIENTS [1]
// mode, waiting for clients to gracefully disconnect [2]. This approach was
// suggested by a PgBouncer maintainer [3] to deal with failovers in Kubernetes.
//
// What happens:
//  1. Operator sends SIGTERM [2] to PgBouncer process (PID 1 in container)
//  2. PgBouncer enters SHUTDOWN WAIT_FOR_CLIENTS mode [1].
//  3. After Kubernetes grace period (default 30s), SIGKILL is sent if process still hasn't exited
//  4. Container is terminated and restarted by Kubernetes Deployment controller.
//  5. New PgBouncer process does fresh DNS lookup → connects to current primary.
//
// This approach is more effective than RECONNECT command for session mode with persistent
// clients (MPG clusters) because RECONNECT waits for clients to disconnect, which never happens
// for persistent clients. SIGTERM will guarantee termination and restarts after a grace period.
//
// [1] https://www.pgbouncer.org/usage.html#shutdown
// [2] https://www.pgbouncer.org/usage.html#signals
// [3] https://github.com/pgbouncer/pgbouncer/issues/1361
func SignalFailover(ctx context.Context, exec postgres.Executor) error {
	log := logging.FromContext(ctx)
	log.Info("SignalFailover: sending SIGTERM to force container restart")

	err := exec(ctx, nil, nil, nil, "kill", "-TERM", "1")

	log.Info("SignalFailover: SIGTERM sent.", "failed", err != nil)
	return err
}
