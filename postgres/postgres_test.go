package postgres_test

import (
	"os"
	"testing"

	"github.com/omeid/conex"
	"github.com/omeid/conex/postgres"
)

func TestMain(m *testing.M) {
	os.Exit(conex.Run(m))
}

func TestPostgres(t *testing.T) {

	sql, con := postgres.Box(t, &postgres.Config{
		Database: "postgres",
		User:     "postgres",
	})
	defer con.Drop()

	var resp int
	err := sql.QueryRow("SELECT 1").Scan(&resp)

	if err != nil {
		t.Fatal(err)
	}

	if resp != 1 {
		t.Fatal("Unexpected response: %v")
	}

}
