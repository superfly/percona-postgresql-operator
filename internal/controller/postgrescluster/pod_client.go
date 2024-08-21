/*
 Copyright 2021 - 2024 Crunchy Data Solutions, Inc.
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package postgrescluster

import (
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/percona/percona-postgresql-operator/internal/postgres"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// podExecutor runs command on container in pod in namespace. Non-nil streams
// (stdin, stdout, and stderr) are attached the to the remote process.
type podExecutor func(
	namespace, pod, container string,
	stdin io.Reader, stdout, stderr io.Writer, command ...string,
) error

func newPodClient(config *rest.Config) (rest.Interface, error) {
	codecs := serializer.NewCodecFactory(scheme.Scheme)
	gvk, _ := apiutil.GVKForObject(&corev1.Pod{}, scheme.Scheme)
	cl, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, err
	}
	return apiutil.RESTClientForGVK(gvk, false, config, codecs, cl)
}

// +kubebuilder:rbac:groups="",resources="pods/exec",verbs={create}

func newPodExecutor(config *rest.Config) (podExecutor, error) {
	client, err := newPodClient(config)

	return func(
		namespace, pod, container string,
		stdin io.Reader, stdout, stderr io.Writer, command ...string,
	) error {
		// FKS doesn't set the env vars properly - https://github.com/superfly/flyio-virtual-kubelet/issues/187
		envVars := map[string]string{
			"KUBERNETES_SERVICE_HOST": "kubernetes.default.svc.cluster.local",
			"KUBERNETES_SERVICE_PORT": "443",
			"PATRONICTL_CONFIG_FILE":  "/etc/patroni",
			//"PGDATA":                  "/pgdata/pg16", // TODO: detect the version
			"PGHOST": postgres.SocketDirectory,
			"PGPORT": "5432",
		}

		exports := ""
		for k, v := range envVars {
			exports += fmt.Sprintf("export %s=%s\n", k, v)
		}

		// convert stdin to string - FKS doesn't support stdin on exec - https://github.com/superfly/flyio-virtual-kubelet/issues/186
		stdinStr := ""
		if stdin != nil {
			buf := new(strings.Builder)
			_, err := io.Copy(buf, stdin)
			if err != nil {
				return err
			}
			stdinStr = buf.String()
		}
		stdinB64 := base64.StdEncoding.EncodeToString([]byte(stdinStr))

		//commandStr := strings.Join(command, " ")
		var commandB64 []string
		for _, c := range command {
			commandB64 = append(commandB64, `"`+base64.StdEncoding.EncodeToString([]byte(c))+`"`)
		}

		runner := []string{"su", "postgres", "-c", fmt.Sprintf(`
echo %s | base64 --decode >/tmp/stdin

command=(
  %s
)

decoded=()

for base64_string in "${command[@]}"; do
    decoded_string=$(echo "$base64_string" | base64 --decode)
    decoded+=("$decoded_string")
done

%s
"${decoded[@]}" </tmp/stdin
`, stdinB64, strings.Join(commandB64, "\n"), exports)}

		logrus.Infof("running cmd in namespace[%s] pod[%s] container[%s]", namespace, pod, container)
		logrus.Infof("command: %v", command)
		logrus.Infof("stdin: %v", stdinStr)
		logrus.Infof("runner: %v", runner)

		request := client.Post().
			Resource("pods").SubResource("exec").
			Namespace(namespace).Name(pod).
			VersionedParams(&corev1.PodExecOptions{
				Container: container,
				Command:   runner,
				Stdin:     false, //stdin != nil,
				Stdout:    stdout != nil,
				Stderr:    stderr != nil,
			}, scheme.ParameterCodec)

		exec, err := remotecommand.NewSPDYExecutor(config, "POST", request.URL())

		if err == nil {
			err = exec.Stream(remotecommand.StreamOptions{
				Stdin:  nil, //stdin,
				Stdout: stdout,
				Stderr: stderr,
			})
		}

		return err
	}, err
}
