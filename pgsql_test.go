package conex_test

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/omeid/conex"

	// The driver.
	_ "github.com/lib/pq"
)

var postgresImage = "postgres:alpine"

func init() {
	conex.Require(func() string { return postgresImage })
}

func TestPGsql1(t *testing.T) { t.Parallel(); testSQLPing(t) }
func TestPGsql2(t *testing.T) { t.Parallel(); testSQLPing(t) }
func TestPGsql3(t *testing.T) { t.Parallel(); testSQLPing(t) }
func TestPGsql4(t *testing.T) { t.Parallel(); testSQLPing(t) }

func testSQLPing(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image:  postgresImage,
		Expose: []string{"5432"},
		Env:    []string{"POSTGRES_HOST_AUTH_METHOD=trust"},
	})
	defer c.Drop()

	t.Log("Waiting for Postgresql to accept connections")
	err := c.Wait("5432", 30*time.Second)
	if err != nil {
		t.Fatal("Postgres failed to start:", err)
	}
	t.Log("Postgresql is now accepting connections")

	connStr := fmt.Sprintf("postgres://postgres@%s:5432/postgres?sslmode=disable", c.Address())
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		t.Fatal(err)
	}
}
