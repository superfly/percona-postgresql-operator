package util

import "fmt"

func WaitUntilInitDone(caller string) string {
	return fmt.Sprintf(`
until [[ -f /fly-init/pg-ready && -f /fly-init/nss-ready ]]; do printf .; sleep 1; done; echo "init done, starting %s";
`, caller)
}
