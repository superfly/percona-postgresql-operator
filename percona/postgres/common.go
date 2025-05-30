package perconaPG

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v2 "github.com/percona/percona-postgresql-operator/pkg/apis/pgv2.percona.com/v2"
)

func GetPrimaryPod(ctx context.Context, cli client.Client, cr *v2.PerconaPGCluster) (*corev1.Pod, error) {
	podList := &corev1.PodList{}
	// K8SPG-648: patroni v4.0.0 deprecated "master" role.
	//            We should use "primary" instead
	err := cli.List(ctx, podList, &client.ListOptions{
		Namespace: cr.Namespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"app.kubernetes.io/instance":                             cr.Name,
			"postgres-operator.crunchydata.com/pgbackrest-dedicated": "",
		}),
	})
	if err != nil {
		return nil, err
	}

	if len(podList.Items) == 0 {
		return nil, errors.New("no repo-host pod found")
	}

	if len(podList.Items) > 1 {
		return nil, errors.New("multiple repo-host pods found")
	}

	return &podList.Items[0], nil
}

func GetRepoHostPod(ctx context.Context, cli client.Client, cr *v2.PerconaPGCluster) (*corev1.Pod, error) {
	podList := &corev1.PodList{}
	err := cli.List(ctx, podList, &client.ListOptions{
		Namespace: cr.Namespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"app.kubernetes.io/instance":             cr.Name,
			"postgres-operator.crunchydata.com/role": "repo-host",
		}),
	})
	if err != nil {
		return nil, err
	}

	if len(podList.Items) == 0 {
		return nil, errors.New("no repo-host pod found")
	}

	if len(podList.Items) > 1 {
		return nil, errors.New("multiple repo-host pods found")
	}

	return &podList.Items[0], nil
}
