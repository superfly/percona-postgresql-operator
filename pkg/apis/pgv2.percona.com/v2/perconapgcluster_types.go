package v2

import (
	"context"
	"fmt"
	"os"

	gover "github.com/hashicorp/go-version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/percona/percona-postgresql-operator/internal/logging"
	pNaming "github.com/percona/percona-postgresql-operator/percona/naming"
	crunchyv1beta1 "github.com/percona/percona-postgresql-operator/pkg/apis/postgres-operator.crunchydata.com/v1beta1"
)

func init() {
	SchemeBuilder.Register(&PerconaPGCluster{}, &PerconaPGClusterList{})
}

const (
	Version     = "2.5.0"
	ProductName = "pg-operator"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=pg
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=".status.host"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=".status.state"
// +kubebuilder:printcolumn:name="Postgres",type=string,JSONPath=".status.postgres.ready"
// +kubebuilder:printcolumn:name="PGBouncer",type=string,JSONPath=".status.pgbouncer.ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +operator-sdk:csv:customresourcedefinitions:order=1
// +operator-sdk:csv:customresourcedefinitions:resources={{ConfigMap,v1},{Secret,v1},{Service,v1},{CronJob,v1beta1},{Deployment,v1},{Job,v1},{StatefulSet,v1},{PersistentVolumeClaim,v1}}
//
// PerconaPGCluster is the CRD that defines a Percona PG Cluster
type PerconaPGCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   PerconaPGClusterSpec   `json:"spec"`
	Status PerconaPGClusterStatus `json:"status,omitempty"`
}

type PerconaPGClusterSpec struct {
	// +optional
	Metadata *crunchyv1beta1.Metadata `json:"metadata,omitempty"`

	// Version of the operator. Update this to new version after operator
	// upgrade to apply changes to Kubernetes objects. Default is the latest
	// version.
	// +optional
	CRVersion string `json:"crVersion,omitempty"`

	// The image name to use for PostgreSQL containers.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=1
	Image string `json:"image,omitempty"`

	// The name of a secret containing environment variables to be set in the PostgreSQL
	// database container, in addition to default environment variables.
	// +optional
	EnvVarsSecret string `json:"envVarsSecret,omitempty"`

	// ImagePullPolicy is used to determine when Kubernetes will attempt to
	// pull (download) container images.
	// More info: https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy
	// +kubebuilder:validation:Enum={Always,Never,IfNotPresent}
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// The image pull secrets used to pull from a private registry
	// Changing this value causes all running pods to restart.
	// https://k8s.io/docs/tasks/configure-pod-container/pull-image-private-registry/
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// The port on which PostgreSQL should listen.
	// +optional
	// +kubebuilder:default=5432
	// +kubebuilder:validation:Minimum=1024
	Port *int32 `json:"port,omitempty"`

	// Specification of the service that exposes the PostgreSQL primary instance.
	// +optional
	Expose *ServiceExpose `json:"expose,omitempty"`

	// Specification of the service that exposes PostgreSQL replica instances
	// +optional
	ExposeReplicas *ServiceExpose `json:"exposeReplicas,omitempty"`

	// The major version of PostgreSQL installed in the PostgreSQL image
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=12
	// +kubebuilder:validation:Maximum=16
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PostgresVersion int `json:"postgresVersion"`

	Secrets SecretsSpec `json:"secrets,omitempty"`

	// Run this cluster as a read-only copy of an existing cluster or archive.
	// +optional
	Standby *crunchyv1beta1.PostgresStandbySpec `json:"standby,omitempty"`

	// Whether or not the PostgreSQL cluster is being deployed to an OpenShift
	// environment. If the field is unset, the operator will automatically
	// detect the environment.
	// +optional
	OpenShift *bool `json:"openshift,omitempty"`

	// +optional
	Patroni *crunchyv1beta1.PatroniSpec `json:"patroni,omitempty"`

	// Users to create inside PostgreSQL and the databases they should access.
	// The default creates one user that can access one database matching the
	// PostgresCluster name. An empty list creates no users. Removing a user
	// from this list does NOT drop the user nor revoke their access.
	// +listType=map
	// +listMapKey=name
	// +optional
	Users []crunchyv1beta1.PostgresUserSpec `json:"users,omitempty"`

	// DatabaseInitSQL defines a ConfigMap containing custom SQL that will
	// be run after the cluster is initialized. This ConfigMap must be in the same
	// namespace as the cluster.
	// +optional
	DatabaseInitSQL *crunchyv1beta1.DatabaseInitSQL `json:"databaseInitSQL,omitempty"`

	// Whether or not the PostgreSQL cluster should be stopped.
	// When this is true, workloads are scaled to zero and CronJobs
	// are suspended.
	// Other resources, such as Services and Volumes, remain in place.
	// +optional
	Pause *bool `json:"pause,omitempty"`

	// Suspends the rollout and reconciliation of changes made to the
	// PostgresCluster spec.
	// +optional
	Unmanaged *bool `json:"unmanaged,omitempty"`

	// Specifies a data source for bootstrapping the PostgreSQL cluster.
	// +optional
	DataSource *crunchyv1beta1.DataSource `json:"dataSource,omitempty"`

	// Specifies one or more sets of PostgreSQL pods that replicate data for
	// this cluster.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	InstanceSets PGInstanceSets `json:"instances"`

	// The specification of a proxy that connects to PostgreSQL.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Proxy *PGProxySpec `json:"proxy,omitempty"`

	// PostgreSQL backup configuration
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Backups Backups `json:"backups"`

	// The specification of PMM sidecars.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	PMM *PMMSpec `json:"pmm,omitempty"`

	// The specification of extensions.
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Extensions ExtensionsSpec `json:"extensions,omitempty"`
}

func (cr *PerconaPGCluster) Default() {
	if len(cr.Spec.CRVersion) == 0 {
		cr.Spec.CRVersion = Version
	}

	for i := range cr.Spec.InstanceSets {
		if cr.Spec.InstanceSets[i].Metadata == nil {
			cr.Spec.InstanceSets[i].Metadata = new(crunchyv1beta1.Metadata)
		}
		if cr.Spec.InstanceSets[i].Metadata.Labels == nil {
			cr.Spec.InstanceSets[i].Metadata.Labels = make(map[string]string)
		}
		cr.Spec.InstanceSets[i].Metadata.Labels[LabelOperatorVersion] = cr.Spec.CRVersion
	}

	if cr.Spec.Proxy == nil {
		cr.Spec.Proxy = new(PGProxySpec)
	}

	if cr.Spec.Proxy.PGBouncer == nil {
		cr.Spec.Proxy.PGBouncer = new(PGBouncerSpec)
	}

	if cr.Spec.Proxy.PGBouncer.Metadata == nil {
		cr.Spec.Proxy.PGBouncer.Metadata = new(crunchyv1beta1.Metadata)
	}
	if cr.Spec.Proxy.PGBouncer.Metadata.Labels == nil {
		cr.Spec.Proxy.PGBouncer.Metadata.Labels = make(map[string]string)
	}
	cr.Spec.Proxy.PGBouncer.Metadata.Labels[LabelOperatorVersion] = cr.Spec.CRVersion

	t := true

	if cr.Spec.Backups.TrackLatestRestorableTime == nil {
		cr.Spec.Backups.TrackLatestRestorableTime = &t
	}
	if cr.Spec.Backups.PGBackRest.Metadata == nil {
		cr.Spec.Backups.PGBackRest.Metadata = new(crunchyv1beta1.Metadata)
	}
	if cr.Spec.Backups.PGBackRest.Metadata.Labels == nil {
		cr.Spec.Backups.PGBackRest.Metadata.Labels = make(map[string]string)
	}
	cr.Spec.Backups.PGBackRest.Metadata.Labels[LabelOperatorVersion] = cr.Spec.CRVersion

	if cr.Spec.Extensions.BuiltIn.PGStatMonitor == nil {
		cr.Spec.Extensions.BuiltIn.PGStatMonitor = &t
	}
	if cr.Spec.Extensions.BuiltIn.PGAudit == nil {
		cr.Spec.Extensions.BuiltIn.PGAudit = &t
	}
}

func (cr *PerconaPGCluster) ToCrunchy(ctx context.Context, postgresCluster *crunchyv1beta1.PostgresCluster, scheme *runtime.Scheme) (*crunchyv1beta1.PostgresCluster, error) {
	log := logging.FromContext(ctx)

	if postgresCluster == nil {
		postgresCluster = &crunchyv1beta1.PostgresCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cr.Name,
				Namespace: cr.Namespace,
			},
		}
	}

	if err := controllerutil.SetControllerReference(cr, postgresCluster, scheme); err != nil {
		return nil, err
	}

	postgresCluster.Default()

	annotations := make(map[string]string)
	for k, v := range cr.Annotations {
		switch k {
		case corev1.LastAppliedConfigAnnotation:
			continue
		default:
			annotations[pNaming.ToCrunchyAnnotation(k)] = v
		}
	}
	postgresCluster.Annotations = annotations
	postgresCluster.Labels = cr.Labels
	if postgresCluster.Labels == nil {
		postgresCluster.Labels = make(map[string]string)
	}
	postgresCluster.Labels[LabelOperatorVersion] = cr.Spec.CRVersion

	postgresCluster.Spec.Metadata = cr.Spec.Metadata
	postgresCluster.Spec.Image = cr.Spec.Image
	postgresCluster.Spec.ImagePullPolicy = cr.Spec.ImagePullPolicy
	postgresCluster.Spec.ImagePullSecrets = cr.Spec.ImagePullSecrets

	postgresCluster.Spec.PostgresVersion = cr.Spec.PostgresVersion
	postgresCluster.Spec.Port = cr.Spec.Port
	postgresCluster.Spec.OpenShift = cr.Spec.OpenShift
	postgresCluster.Spec.Paused = cr.Spec.Unmanaged
	postgresCluster.Spec.Shutdown = cr.Spec.Pause
	postgresCluster.Spec.Standby = cr.Spec.Standby
	postgresCluster.Spec.Service = cr.Spec.Expose.ToCrunchy()
	postgresCluster.Spec.ReplicaService = cr.Spec.ExposeReplicas.ToCrunchy()

	postgresCluster.Spec.CustomReplicationClientTLSSecret = cr.Spec.Secrets.CustomReplicationClientTLSSecret
	postgresCluster.Spec.CustomTLSSecret = cr.Spec.Secrets.CustomTLSSecret
	postgresCluster.Spec.CustomRootCATLSSecret = cr.Spec.Secrets.CustomRootCATLSSecret

	postgresCluster.Spec.Backups = cr.Spec.Backups.ToCrunchy(cr.Spec.CRVersion)
	for i := range postgresCluster.Spec.Backups.PGBackRest.Repos {
		repo := postgresCluster.Spec.Backups.PGBackRest.Repos[i]

		if repo.BackupSchedules == nil {
			continue
		}
		repo.BackupSchedules.Differential = nil
		repo.BackupSchedules.Full = nil
		repo.BackupSchedules.Incremental = nil
	}
	postgresCluster.Spec.DataSource = cr.Spec.DataSource
	postgresCluster.Spec.DatabaseInitSQL = cr.Spec.DatabaseInitSQL
	postgresCluster.Spec.Patroni = cr.Spec.Patroni

	users := make([]crunchyv1beta1.PostgresUserSpec, 0)

	for _, user := range cr.Spec.Users {
		if user.Name == UserMonitoring {
			log.Info(UserMonitoring + " user is reserved, it'll be ignored.")
			continue
		}
		users = append(users, user)
	}

	if cr.PMMEnabled() {
		users = append(cr.Spec.Users, crunchyv1beta1.PostgresUserSpec{
			Name:    UserMonitoring,
			Options: "SUPERUSER",
			Password: &crunchyv1beta1.PostgresPasswordSpec{
				Type: crunchyv1beta1.PostgresPasswordTypeAlphaNumeric,
			},
		})

		if len(cr.Spec.Users) == 0 {
			// Add default user: <cluster-name>-pguser-<cluster-name>
			users = append(users, crunchyv1beta1.PostgresUserSpec{
				Name: crunchyv1beta1.PostgresIdentifier(cr.Name),
				Databases: []crunchyv1beta1.PostgresIdentifier{
					crunchyv1beta1.PostgresIdentifier(cr.Name),
				},
				Password: &crunchyv1beta1.PostgresPasswordSpec{
					Type: crunchyv1beta1.PostgresPasswordTypeAlphaNumeric,
				},
			})
		}

		postgresCluster.Spec.Extensions.PGStatStatements = cr.Spec.PMM.QuerySource == PgStatStatements
	}

	postgresCluster.Spec.Users = users

	postgresCluster.Spec.InstanceSets = cr.Spec.InstanceSets.ToCrunchy()
	postgresCluster.Spec.Proxy = cr.Spec.Proxy.ToCrunchy()

	postgresCluster.Spec.Extensions.PGStatMonitor = *cr.Spec.Extensions.BuiltIn.PGStatMonitor
	postgresCluster.Spec.Extensions.PGAudit = *cr.Spec.Extensions.BuiltIn.PGAudit

	// Add annotations for environment variables from secrets
	if cr.Spec.EnvVarsSecret != "" {
		if postgresCluster.Annotations == nil {
			postgresCluster.Annotations = make(map[string]string)
		}
		postgresCluster.Annotations["percona.com/env-vars-secret"] = cr.Spec.EnvVarsSecret
	}

	// Add annotations for instance-specific environment variables from secrets
	for _, instance := range cr.Spec.InstanceSets {
		if instance.EnvVarsSecret != "" {
			if postgresCluster.Annotations == nil {
				postgresCluster.Annotations = make(map[string]string)
			}
			postgresCluster.Annotations[fmt.Sprintf("percona.com/instance-%s-env-vars-secret", instance.Name)] = instance.EnvVarsSecret
		}
	}

	// Add annotation for pgBackRest repo host environment variables from secret
	if cr.Spec.Backups.PGBackRest.RepoHost != nil && cr.Spec.Backups.PGBackRest.RepoHost.EnvVarsSecret != "" {
		if postgresCluster.Annotations == nil {
			postgresCluster.Annotations = make(map[string]string)
		}
		postgresCluster.Annotations["percona.com/pgbackrest-repo-host-env-vars-secret"] = cr.Spec.Backups.PGBackRest.RepoHost.EnvVarsSecret
	}

	return postgresCluster, nil
}

func (cr *PerconaPGCluster) Version() *gover.Version {
	return gover.Must(gover.NewVersion(cr.Spec.CRVersion))
}

func (cr *PerconaPGCluster) CompareVersion(ver string) int {
	return cr.Version().Compare(gover.Must(gover.NewVersion(ver)))
}

type AppState string

const (
	AppStateInit     AppState = "initializing"
	AppStatePaused   AppState = "paused"
	AppStateStopping AppState = "stopping"
	AppStateReady    AppState = "ready"
	AppStateError    AppState = "error"
)

type PostgresInstanceSetStatus struct {
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	Size int32 `json:"size"`

	// +kubebuilder:validation:Required
	Ready int32 `json:"ready"`
}

type PostgresStatus struct {
	// +kubebuilder:validation:Required
	Size int32 `json:"size"`

	// +kubebuilder:validation:Required
	Ready int32 `json:"ready"`

	// +kubebuilder:validation:Required
	InstanceSets []PostgresInstanceSetStatus `json:"instances"`
}

type PGBouncerStatus struct {
	// +kubebuilder:validation:Required
	Size int32 `json:"size"`

	// +kubebuilder:validation:Required
	Ready int32 `json:"ready"`
}

type PerconaPGClusterStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Postgres PostgresStatus `json:"postgres"`

	// +operator-sdk:csv:customresourcedefinitions:type=status
	PGBouncer PGBouncerStatus `json:"pgbouncer"`

	// +operator-sdk:csv:customresourcedefinitions:type=status
	State AppState `json:"state"`

	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Host string `json:"host"`
}

type Backups struct {
	// pgBackRest archive configuration
	// +kubebuilder:validation:Required
	PGBackRest PGBackRestArchive `json:"pgbackrest"`

	// Enable tracking latest restorable time
	TrackLatestRestorableTime *bool `json:"trackLatestRestorableTime,omitempty"`
}

func (b Backups) ToCrunchy(version string) crunchyv1beta1.Backups {
	var sc *crunchyv1beta1.PGBackRestSidecars
	if b.PGBackRest.Containers != nil {
		sc = b.PGBackRest.Containers
	} else if b.PGBackRest.Sidecars != nil {
		sc = b.PGBackRest.Sidecars
	}

	return crunchyv1beta1.Backups{
		PGBackRest: crunchyv1beta1.PGBackRestArchive{
			Metadata:      b.PGBackRest.Metadata,
			Configuration: b.PGBackRest.Configuration,
			Global:        b.PGBackRest.Global,
			Image:         b.PGBackRest.Image,
			Jobs:          b.PGBackRest.Jobs,
			Repos:         b.PGBackRest.Repos,
			RepoHost:      b.PGBackRest.RepoHost.ToCrunchy(),
			Manual:        b.PGBackRest.Manual,
			Restore:       b.PGBackRest.Restore,
			Sidecars:      sc,
		},
	}
}

type PGBackRestArchive struct {
	// +optional
	Metadata *crunchyv1beta1.Metadata `json:"metadata,omitempty"`

	// Projected volumes containing custom pgBackRest configuration.  These files are mounted
	// under "/etc/pgbackrest/conf.d" alongside any pgBackRest configuration generated by the
	// PostgreSQL Operator:
	// https://pgbackrest.org/configuration.html
	// +optional
	Configuration []corev1.VolumeProjection `json:"configuration,omitempty"`

	// Global pgBackRest configuration settings.  These settings are included in the "global"
	// section of the pgBackRest configuration generated by the PostgreSQL Operator, and then
	// mounted under "/etc/pgbackrest/conf.d":
	// https://pgbackrest.org/configuration.html
	// +optional
	Global map[string]string `json:"global,omitempty"`

	// The image name to use for pgBackRest containers.  Utilized to run
	// pgBackRest repository hosts and backups. The image may also be set using
	// the RELATED_IMAGE_PGBACKREST environment variable
	// +optional
	Image string `json:"image,omitempty"`

	// Jobs field allows configuration for all backup jobs
	// +optional
	Jobs *crunchyv1beta1.BackupJobs `json:"jobs,omitempty"`

	// Defines a pgBackRest repository
	// +kubebuilder:validation:MinItems=1
	// +listType=map
	// +listMapKey=name
	Repos []crunchyv1beta1.PGBackRestRepo `json:"repos"`

	// Defines configuration for a pgBackRest dedicated repository host.  This section is only
	// applicable if at least one "volume" (i.e. PVC-based) repository is defined in the "repos"
	// section, therefore enabling a dedicated repository host Deployment.
	// +optional
	RepoHost *PGBackRestRepoHost `json:"repoHost,omitempty"`

	// Defines details for manual pgBackRest backup Jobs
	// +optional
	Manual *crunchyv1beta1.PGBackRestManualBackup `json:"manual,omitempty"`

	// Defines details for performing an in-place restore using pgBackRest
	// +optional
	Restore *crunchyv1beta1.PGBackRestRestore `json:"restore,omitempty"`

	// Deprecated: Use Containers instead
	// +optional
	Sidecars *crunchyv1beta1.PGBackRestSidecars `json:"sidecars,omitempty"`

	// Configuration for pgBackRest sidecar containers
	// +optional
	Containers *crunchyv1beta1.PGBackRestSidecars `json:"containers,omitempty"`
}

type PMMQuerySource string

const (
	PgStatMonitor    PMMQuerySource = "pgstatmonitor"
	PgStatStatements PMMQuerySource = "pgstatstatements"
)

type PMMSpec struct {
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// ImagePullPolicy is used to determine when Kubernetes will attempt to
	// pull (download) container images.
	// More info: https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy
	// +kubebuilder:validation:Enum={Always,Never,IfNotPresent}
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// +kubebuilder:validation:Required
	ServerHost string `json:"serverHost,omitempty"`

	// +kubebuilder:validation:Required
	Secret string `json:"secret,omitempty"`

	// +kubebuilder:validation:Enum={pgstatmonitor,pgstatstatements}
	// +kubebuilder:default=pgstatmonitor
	// +kubebuilder:validation:Required
	QuerySource PMMQuerySource `json:"querySource,omitempty"`

	// Compute resources of a PMM container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// +optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`

	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`
}

func (cr *PerconaPGCluster) PMMEnabled() bool {
	return cr.Spec.PMM != nil && cr.Spec.PMM.Enabled
}

type CustomExtensionSpec struct {
	Name     string `json:"name,omitempty"`
	Version  string `json:"version,omitempty"`
	Checksum string `json:"checksum,omitempty"`
}

type CustomExtensionsStorageSpec struct {
	// +kubebuilder:validation:Enum={s3,gcs,azure}
	Type     string                   `json:"type,omitempty"`
	Bucket   string                   `json:"bucket,omitempty"`
	Region   string                   `json:"region,omitempty"`
	Endpoint string                   `json:"endpoint,omitempty"`
	Secret   *corev1.SecretProjection `json:"secret,omitempty"`
}

type BuiltInExtensionsSpec struct {
	PGStatMonitor *bool `json:"pg_stat_monitor,omitempty"`
	PGAudit       *bool `json:"pg_audit,omitempty"`
}

type ExtensionsSpec struct {
	// +kubebuilder:validation:Required
	Image           string                      `json:"image"`
	ImagePullPolicy corev1.PullPolicy           `json:"imagePullPolicy,omitempty"`
	Storage         CustomExtensionsStorageSpec `json:"storage,omitempty"`
	BuiltIn         BuiltInExtensionsSpec       `json:"builtin,omitempty"`
	Custom          []CustomExtensionSpec       `json:"custom,omitempty"`
}

type SecretsSpec struct {
	// The secret containing the Certificates and Keys to encrypt PostgreSQL
	// traffic will need to contain the server TLS certificate, TLS key and the
	// Certificate Authority certificate with the data keys set to tls.crt,
	// tls.key and ca.crt, respectively. It will then be mounted as a volume
	// projection to the '/pgconf/tls' directory. For more information on
	// Kubernetes secret projections, please see
	// https://k8s.io/docs/concepts/configuration/secret/#projection-of-secret-keys-to-specific-paths
	// NOTE: If CustomTLSSecret is provided, CustomReplicationClientTLSSecret
	// MUST be provided and the ca.crt provided must be the same.
	// +optional
	CustomTLSSecret *corev1.SecretProjection `json:"customTLSSecret,omitempty"`

	// The secret containing the replication client certificates and keys for
	// secure connections to the PostgreSQL server. It will need to contain the
	// client TLS certificate, TLS key and the Certificate Authority certificate
	// with the data keys set to tls.crt, tls.key and ca.crt, respectively.
	// NOTE: If CustomReplicationClientTLSSecret is provided, CustomTLSSecret
	// MUST be provided and the ca.crt provided must be the same.
	// +optional
	CustomReplicationClientTLSSecret *corev1.SecretProjection `json:"customReplicationTLSSecret,omitempty"`

	// The secret containing the root CA certificate and key for
	// secure connections to the PostgreSQL server. It will need to contain the
	// CA TLS certificate and CA TLS key with the data keys set to
	// root.crt and root.key, respectively.
	// +optional
	CustomRootCATLSSecret *corev1.SecretProjection `json:"customRootCATLSSecret,omitempty"`
}

// +listType=map
// +listMapKey=name
// +kubebuilder:validation:MinItems=1
type PGInstanceSets []PGInstanceSetSpec

func (p PGInstanceSets) ToCrunchy() []crunchyv1beta1.PostgresInstanceSetSpec {
	set := make([]crunchyv1beta1.PostgresInstanceSetSpec, len(p))

	for i, inst := range p {
		set[i] = inst.ToCrunchy()
	}

	return set
}

type PGInstanceSetSpec struct {
	// +optional
	Metadata *crunchyv1beta1.Metadata `json:"metadata,omitempty"`

	// This value goes into the name of an appsv1.StatefulSet, the hostname of
	// a corev1.Pod, and label values. The pattern below is IsDNS1123Label
	// wrapped in "()?" to accommodate the empty default.
	//
	// The Pods created by a StatefulSet have a "controller-revision-hash" label
	// comprised of the StatefulSet name, a dash, and a 10-character hash.
	// The length below is derived from limitations on label values:
	//
	//   63 (max) ≥ len(cluster) + 1 (dash)
	//                + len(set) + 1 (dash) + 4 (id)
	//                + 1 (dash) + 10 (hash)
	//
	// See: https://issue.k8s.io/64023

	// Name that associates this set of PostgreSQL pods. This field is optional
	// when only one instance set is defined. Each instance set in a cluster
	// must have a unique name. The combined length of this and the cluster name
	// must be 46 characters or less.
	// +optional
	// +kubebuilder:default=""
	// +kubebuilder:validation:Pattern=`^([a-z0-9]([-a-z0-9]*[a-z0-9])?)?$`
	Name string `json:"name"`

	// Scheduling constraints of a PostgreSQL pod. Changing this value causes
	// PostgreSQL to restart.
	// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Custom sidecars for PostgreSQL instance pods. Changing this value causes
	// PostgreSQL to restart.
	// +optional
	Sidecars []corev1.Container `json:"sidecars,omitempty"`

	// Configuration for instance default sidecar containers.
	// +optional
	Containers *crunchyv1beta1.InstanceSidecars `json:"containers,omitempty"`

	// Additional init containers for PostgreSQL instance pods. Changing this value causes
	// PostgreSQL to restart.
	// +optional
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// Priority class name for the PostgreSQL pod. Changing this value causes
	// PostgreSQL to restart.
	// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/
	// +optional
	PriorityClassName *string `json:"priorityClassName,omitempty"`

	// Number of desired PostgreSQL pods.
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	Replicas *int32 `json:"replicas,omitempty"`

	// Minimum number of pods that should be available at a time.
	// Defaults to one when the replicas field is greater than one.
	// +optional
	MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty"`

	// Compute resources of a PostgreSQL container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Tolerations of a PostgreSQL pod. Changing this value causes PostgreSQL to restart.
	// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Topology spread constraints of a PostgreSQL pod. Changing this value causes
	// PostgreSQL to restart.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// Defines a separate PersistentVolumeClaim for PostgreSQL's write-ahead log.
	// More info: https://www.postgresql.org/docs/current/wal.html
	// +optional
	WALVolumeClaimSpec *corev1.PersistentVolumeClaimSpec `json:"walVolumeClaimSpec,omitempty"`

	// Defines a PersistentVolumeClaim for PostgreSQL data.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes
	// +kubebuilder:validation:Required
	DataVolumeClaimSpec corev1.PersistentVolumeClaimSpec `json:"dataVolumeClaimSpec"`

	// The name of a secret containing environment variables to be set in the PostgreSQL
	// database container, in addition to default environment variables.
	// If specified, this overrides the cluster-level envVarsSecret.
	// +optional
	EnvVarsSecret string `json:"envVarsSecret,omitempty"`

	// The list of tablespaces volumes to mount for this postgrescluster
	// This field requires enabling TablespaceVolumes feature gate
	// +listType=map
	// +listMapKey=name
	// +optional
	TablespaceVolumes []crunchyv1beta1.TablespaceVolume `json:"tablespaceVolumes,omitempty"`

	// The list of volume mounts to mount to PostgreSQL instance pods. Changing this value causes
	// PostgreSQL to restart.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// SecurityContext defines the security settings for a PostgreSQL pod.
	// +optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`
}

func (p PGInstanceSetSpec) ToCrunchy() crunchyv1beta1.PostgresInstanceSetSpec {
	return crunchyv1beta1.PostgresInstanceSetSpec{
		Metadata:                  p.Metadata,
		Name:                      p.Name,
		Affinity:                  p.Affinity,
		Containers:                p.Sidecars,
		InitContainers:            p.InitContainers,
		WALVolumeClaimSpec:        p.WALVolumeClaimSpec,
		DataVolumeClaimSpec:       p.DataVolumeClaimSpec,
		PriorityClassName:         p.PriorityClassName,
		Replicas:                  p.Replicas,
		MinAvailable:              p.MinAvailable,
		Resources:                 p.Resources,
		Tolerations:               p.Tolerations,
		TopologySpreadConstraints: p.TopologySpreadConstraints,
		VolumeMounts:              p.VolumeMounts,
		SecurityContext:           p.SecurityContext,
		TablespaceVolumes:         p.TablespaceVolumes,
		Sidecars:                  p.Containers,
	}
}

type ServiceExpose struct {
	crunchyv1beta1.Metadata `json:",inline"`

	// The port on which this service is exposed when type is NodePort or
	// LoadBalancer. Value must be in-range and not in use or the operation will
	// fail. If unspecified, a port will be allocated if this Service requires one.
	// - https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport
	// +optional
	NodePort *int32 `json:"nodePort,omitempty"`

	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types
	//
	// +optional
	// +kubebuilder:default=ClusterIP
	// +kubebuilder:validation:Enum={ClusterIP,NodePort,LoadBalancer}
	Type string `json:"type,omitempty"`

	// LoadBalancerSourceRanges is a list of IP CIDRs allowed access to load.
	// This field will be ignored if the cloud-provider does not support the feature.
	// +optional
	LoadBalancerSourceRanges []string `json:"loadBalancerSourceRanges,omitempty"`
}

func (s *ServiceExpose) ToCrunchy() *crunchyv1beta1.ServiceSpec {
	if s == nil {
		return nil
	}

	return &crunchyv1beta1.ServiceSpec{
		Metadata: &crunchyv1beta1.Metadata{
			Annotations: s.Annotations,
			Labels:      s.Labels,
		},
		NodePort:                 s.NodePort,
		Type:                     s.Type,
		LoadBalancerSourceRanges: s.LoadBalancerSourceRanges,
	}
}

type PGProxySpec struct {
	// Defines a PgBouncer proxy and connection pooler.
	PGBouncer *PGBouncerSpec `json:"pgBouncer"`
}

func (p *PGProxySpec) ToCrunchy() *crunchyv1beta1.PostgresProxySpec {
	if p == nil {
		return nil
	}

	return &crunchyv1beta1.PostgresProxySpec{
		PGBouncer: p.PGBouncer.ToCrunchy(),
	}
}

type PGBouncerSpec struct {
	// +optional
	Metadata *crunchyv1beta1.Metadata `json:"metadata,omitempty"`

	// Scheduling constraints of a PgBouncer pod. Changing this value causes
	// PgBouncer to restart.
	// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Configuration settings for the PgBouncer process. Changes to any of these
	// values will be automatically reloaded without validation. Be careful, as
	// you may put PgBouncer into an unusable state.
	// More info: https://www.pgbouncer.org/usage.html#reload
	// +optional
	Config crunchyv1beta1.PGBouncerConfiguration `json:"config,omitempty"`

	// Custom sidecars for a PgBouncer pod. Changing this value causes
	// PgBouncer to restart.
	// +optional
	Sidecars []corev1.Container `json:"sidecars,omitempty"`

	// Configuration for pgBouncer default sidecar containers.
	// +optional
	Containers *crunchyv1beta1.PGBouncerSidecars `json:"containers,omitempty"`

	// A secret projection containing a certificate and key with which to encrypt
	// connections to PgBouncer. The "tls.crt", "tls.key", and "ca.crt" paths must
	// be PEM-encoded certificates and keys. Changing this value causes PgBouncer
	// to restart.
	// More info: https://kubernetes.io/docs/concepts/configuration/secret/#projection-of-secret-keys-to-specific-paths
	// +optional
	CustomTLSSecret *corev1.SecretProjection `json:"customTLSSecret,omitempty"`

	// Allow SUPERUSERs to connect through PGBouncer.
	// +optional
	ExposeSuperusers bool `json:"exposeSuperusers,omitempty"`

	// Name of a container image that can run PgBouncer 1.15 or newer. Changing
	// this value causes PgBouncer to restart. The image may also be set using
	// the RELATED_IMAGE_PGBOUNCER environment variable.
	// More info: https://kubernetes.io/docs/concepts/containers/images
	// +optional
	Image string `json:"image,omitempty"`

	// Port on which PgBouncer should listen for client connections. Changing
	// this value causes PgBouncer to restart.
	// +optional
	// +kubebuilder:default=5432
	// +kubebuilder:validation:Minimum=1024
	Port *int32 `json:"port,omitempty"`

	// Priority class name for the pgBouncer pod. Changing this value causes
	// PostgreSQL to restart.
	// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/
	// +optional
	PriorityClassName *string `json:"priorityClassName,omitempty"`

	// Number of desired PgBouncer pods.
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// Minimum number of pods that should be available at a time.
	// Defaults to one when the replicas field is greater than one.
	// +optional
	MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty"`

	// Compute resources of a PgBouncer container. Changing this value causes
	// PgBouncer to restart.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Specification of the service that exposes PgBouncer.
	// +optional
	ServiceExpose *ServiceExpose `json:"expose,omitempty"`

	// Tolerations of a PgBouncer pod. Changing this value causes PgBouncer to
	// restart.
	// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Topology spread constraints of a PgBouncer pod. Changing this value causes
	// PgBouncer to restart.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// SecurityContext defines the security settings for PGBouncer pods.
	// +optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`
}

func (p *PGBouncerSpec) ToCrunchy() *crunchyv1beta1.PGBouncerPodSpec {
	if p == nil {
		return nil
	}

	spec := &crunchyv1beta1.PGBouncerPodSpec{
		Metadata:                  p.Metadata,
		Affinity:                  p.Affinity,
		Config:                    p.Config,
		Containers:                p.Sidecars,
		Sidecars:                  p.Containers,
		CustomTLSSecret:           p.CustomTLSSecret,
		ExposeSuperusers:          p.ExposeSuperusers,
		Image:                     p.Image,
		Port:                      p.Port,
		PriorityClassName:         p.PriorityClassName,
		Replicas:                  p.Replicas,
		MinAvailable:              p.MinAvailable,
		Resources:                 p.Resources,
		Service:                   p.ServiceExpose.ToCrunchy(),
		Tolerations:               p.Tolerations,
		TopologySpreadConstraints: p.TopologySpreadConstraints,
		SecurityContext:           p.SecurityContext,
	}

	spec.Default()

	return spec
}

// +kubebuilder:object:root=true
// PostgresClusterList contains a list of PostgresCluster
type PerconaPGClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PerconaPGCluster `json:"items"`
}

const labelPrefix = "pgv2.percona.com/"

const (
	LabelOperatorVersion = labelPrefix + "version"
	LabelPMMSecret       = labelPrefix + "pmm-secret"
)

const DefaultVersionServiceEndpoint = "https://check.percona.com"

func GetDefaultVersionServiceEndpoint() string {
	endpoint := os.Getenv("PERCONA_VS_FALLBACK_URI")

	if len(endpoint) != 0 {
		return endpoint
	}

	return DefaultVersionServiceEndpoint
}

const (
	FinalizerDeletePVC    = "percona.com/delete-pvc"
	FinalizerDeleteSSL    = "percona.com/delete-ssl"
	FinalizerStopWatchers = "percona.com/stop-watchers" //nolint:gosec
)

const (
	UserMonitoring = "monitor"
)

type PGBackRestRepoHost struct {
	// Scheduling constraints of the Dedicated repo host pod.
	// Changing this value causes repo host to restart.
	// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Priority class name for the pgBackRest repo host pod. Changing this value
	// causes PostgreSQL to restart.
	// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/
	// +optional
	PriorityClassName *string `json:"priorityClassName,omitempty"`

	// Resource requirements for a pgBackRest repository host
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Tolerations of a PgBackRest repo host pod. Changing this value causes a restart.
	// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Topology spread constraints of a Dedicated repo host pod. Changing this
	// value causes the repo host to restart.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// The name of a secret containing environment variables to be set in the pgBackRest
	// repository host container. This replaces the direct environment variable specification.
	// +optional
	EnvVarsSecret string `json:"envVarsSecret,omitempty"`

	// Defines sidecars that will be added to the repository host pod
	// +optional
	Sidecars []corev1.Container `json:"sidecars,omitempty"`

	// ConfigMap containing custom SSH configuration.
	// Deprecated: Repository hosts use mTLS for encryption, authentication, and authorization.
	// +optional
	SSHConfiguration *corev1.ConfigMapProjection `json:"sshConfigMap,omitempty"`

	// Secret containing custom SSH keys.
	// Deprecated: Repository hosts use mTLS for encryption, authentication, and authorization.
	// +optional
	SSHSecret *corev1.SecretProjection `json:"sshSecret,omitempty"`

	// SecurityContext defines the security settings for PGBackRest pod.
	// +optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`
}

// ToCrunchy converts the Percona PGBackRestRepoHost to a Crunchy PGBackRestRepoHost
func (r *PGBackRestRepoHost) ToCrunchy() *crunchyv1beta1.PGBackRestRepoHost {
	if r == nil {
		return nil
	}

	return &crunchyv1beta1.PGBackRestRepoHost{
		Affinity:                  r.Affinity,
		PriorityClassName:         r.PriorityClassName,
		Resources:                 r.Resources,
		Tolerations:               r.Tolerations,
		TopologySpreadConstraints: r.TopologySpreadConstraints,
		Environment:               []corev1.EnvVar{}, // Empty since we're using EnvVarsSecret
		Sidecars:                  r.Sidecars,
		SSHConfiguration:          r.SSHConfiguration,
		SSHSecret:                 r.SSHSecret,
		SecurityContext:           r.SecurityContext,
	}
}
