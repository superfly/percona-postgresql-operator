# Error Reporting with Sentry

The Percona PostgreSQL Operator supports error reporting through Sentry. This helps track and monitor errors that occur during the operation of your PostgreSQL clusters.

## Configuration

To enable Sentry error reporting:

1. Create a Sentry project and obtain your DSN (Data Source Name)

2. Create a secret containing your Sentry DSN:
   ```bash
   kubectl create secret generic percona-postgresql-operator-sentry \
     --from-literal=dsn=your-sentry-dsn
   ```

3. The operator deployment automatically picks up the Sentry configuration from the secret.

## Environment Variables

The following environment variables can be configured:

- `SENTRY_DSN`: The Sentry DSN (configured via secret)
- `SENTRY_ENVIRONMENT`: The environment name (defaults to namespace name)
- `SENTRY_DEBUG`: Enable debug mode for Sentry (default: "false")

## Error Tracking

The operator reports various types of errors to Sentry:

- Reconciliation errors
- Unexpected errors during cluster operations
- Panics (which are captured and reported before re-panicking)

Each error report includes relevant context such as:
- Namespace and name of the affected PostgreSQL cluster
- Operation being performed
- Controller name
- Additional error context

## Disabling Error Reporting

To disable Sentry error reporting, simply delete the Sentry secret:

```bash
kubectl delete secret percona-postgresql-operator-sentry
```