package pgbackrestserver

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/superfly/percona-postgresql-operator/internal/pgbackrest-server/api"
)

func main() {
	r := mux.NewRouter()
	r = r.PathPrefix("/pgbackrest").Subrouter()

	// Endpoints
	r.HandleFunc("/info", api.InfoCommandHandler).Methods("POST")
	r.HandleFunc("/backup", api.BackupCommandHandler).Methods("POST")
	r.HandleFunc("/restore", api.RestoreCommandHandler).Methods("POST")

	http.Handle("/", r)
	err := http.ListenAndServe(":4422", nil)
	if err != nil {
		panic(fmt.Sprintf("server failed: %v", err))
	}
}
