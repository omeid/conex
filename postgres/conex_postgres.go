package postgres

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/omeid/conex"
	// The driver.
	_ "github.com/lib/pq"
)

var (
	// Image to use for the box.
	Image = "postgres:alpine"
	// Port used for connect to redis.
	Port = "5432"

	// PostgresUpWaitTime dectiates how long we should wait for post Postgresql to accept connections on {{Port}}.
	PostgresUpWaitTime = 10 * time.Second
)

func init() {
	conex.Require(func() string { return Image })
}

// Config used to connect to the database.
type Config struct {
	User     string
	Password string
	Database string // defaults to `postgres` as service db.

	host string
	port string
}

func (c *Config) url() string {

	if c.Database == "" {
		c.Database = "postgres"
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.User, c.Password, c.host, c.port, c.Database,
	)
}

// Box returns an sql.DB connection and the container running the Postgresql
// instance. It will call t.Fatal on errors.
func Box(t testing.TB, config *Config) (*sql.DB, conex.Container) {
	c := conex.Box(t, &conex.Config{
		Image:  Image,
		Expose: []string{Port},
	})

	config.host = c.Address()
	config.port = Port

	t.Logf("Waiting for Postgresql to accept connections")

	err := c.Wait(Port, PostgresUpWaitTime)

	if err != nil {
		c.Drop() // return the container
		t.Fatal("Postgres failed to start.", err)
	}

	t.Log("Postgresql is now accepting connections")
	db, err := sql.Open("postgres", config.url())

	if err != nil {
		c.Drop() // return the container
		t.Fatal(err)
	}

	return db, c
}
