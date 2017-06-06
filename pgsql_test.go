package conex_test

import (
	"testing"

	"github.com/conex/postgres"
)

func TestPGsql1(t *testing.T) { t.Parallel(); testSQLPing(t) }
func TestPGsql2(t *testing.T) { t.Parallel(); testSQLPing(t) }
func TestPGsql3(t *testing.T) { t.Parallel(); testSQLPing(t) }
func TestPGsql4(t *testing.T) { t.Parallel(); testSQLPing(t) }

func testSQLPing(t *testing.T) {

	config := &postgres.Config{
		User:     "postgres",
		Password: "",
	}

	db, c := postgres.Box(t, config)
	_ = c
	defer c.Drop()

	err := db.Ping()

	if err != nil {
		t.Fatal(err)
	}

}
