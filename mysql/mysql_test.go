package mysql_test

import (
	"os"
	"testing"

	"github.com/omeid/conex"
	"github.com/omeid/conex/mysql"
)

func TestMain(m *testing.M) {
	os.Exit(conex.Run(m))
}

func TestMySQL(t *testing.T) {
	sql, con := mysql.Box(t, &mysql.Config{})
	defer con.Drop()

	var resp int
	if err := sql.QueryRow("SELECT 1").Scan(&resp); err != nil {
		t.Fatal(err)
	}

	if resp != 1 {
		t.Fatalf("Unexpected response: %v", resp)
	}
}
