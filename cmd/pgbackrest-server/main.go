package pgbackrestserver

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/superfly/percona-postgresql-operator/cmd/pgbackrest-server/api"
)

func main() {
	r := mux.NewRouter()
	r = r.PathPrefix("/pgbackrest").Subrouter()

	// Endpoints
	r.HandleFunc("/info", api.InfoCommandHandler)

	http.Handle("/", r)
	err := http.ListenAndServe(":4422", nil)
	if err != nil {
		panic(fmt.Sprintf("server failed: %v", err))
	}
}
