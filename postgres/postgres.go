package postgres

import (
	"database/sql"
	"fmt"
	"net"
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
)

func init() {
	conex.Require(func() string { return Image })
}

// Config used to connect to the database.
type Config struct {
	User     string
	Password string
	Database string

	host string
	port string
}

func (c *Config) url() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.User, c.Password, c.host, c.port, c.Database,
	)
}

// Box returns an echo client connect to an echo container based on
// your provided tags.
func Box(t testing.TB, config *Config) (*sql.DB, conex.Container) {
	c := conex.Box(t, &conex.Config{
		Image:  Image,
		Expose: []string{Port},
	})

	config.host = c.Address()
	config.port = Port

	addr := fmt.Sprintf("%s:%s", config.host, config.port)

	var ok bool

	t.Logf("Waiting for Postgrestions to accept connections")
	for i := 0; i < 20; i++ {

		conn, err := net.Dial("tcp", addr)
		if err == nil {
			err = conn.Close()
			if err != nil {
				t.Fatal(err)
			}
			ok = true
			break
		}

		time.Sleep(time.Second)
	}

	if !ok {
		t.Fatal("Postgres failed to start.")
	}

	t.Logf("\n Postgres is up. Now connecting.\n")
	db, err := sql.Open("postgres", config.url())

	if err != nil {
		t.Fatal(err)
	}

	return db, c
}
