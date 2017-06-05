package mysql

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/omeid/conex"
)

var (
	// Image to use for the box.
	Image = "mysql:latest"
	// Port used for connect to MySQL.
	Port = "3306"

	// MySQLUpWaitTime dictates how long we should wait for post MySQL to accept connections on {{Port}}.
	MySQLUpWaitTime = 10 * time.Second
)

func init() {
	conex.Require(func() string { return Image })
}

// Config used to connect to the database.
type Config struct {
	User     string // leave blank for root, otherwise provide a password
	Password string // can be blank for root user
	Database string // defaults to `test`

	host string
	port string
}

// [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
func (c *Config) url() string {
	pass := ""
	if c.Password != "" {
		pass = ":" + c.Password
	}

	return fmt.Sprintf(
		"%s%s@tcp(%s:%s)/%s?autocommit=true",
		c.User, pass, c.host, c.port, c.Database,
	)
}

// set env variables, see https://hub.docker.com/r/library/mysql/
func (c *Config) env() []string {
	env := []string{}

	// set up user/password access
	if c.User == "" {
		c.User = "root"
	}

	if c.User == "root" && c.Password == "" {
		env = append(env, fmt.Sprintf("MYSQL_ALLOW_EMPTY_PASSWORD=yes"))
	} else {
		env = append(env, fmt.Sprintf("MYSQL_PASSWORD=%s", c.Password))
	}

	if c.User != "" && c.User != "root" {
		if c.Password == "" {
			panic("MySQL requires both user and password when a non-root user is specified.")
		}
		env = append(env, fmt.Sprintf("MYSQL_USER=%s", c.User))
		env = append(env, fmt.Sprintf("MYSQL_PASSWORD=%s", c.Password))
	}

	// create a database or default to "test"
	dbName := c.Database
	if dbName == "" {
		dbName = "test"
	}
	env = append(env, fmt.Sprintf("MYSQL_DATABASE=%s", dbName))

	return env
}

// Box returns an echo client connect to an echo container based on your provided tags.
func Box(t testing.TB, config *Config) (*sql.DB, conex.Container) {
	c := conex.Box(t, &conex.Config{
		Image:  Image,
		Env:    config.env(),
		Expose: []string{Port},
	})

	config.host = c.Address()
	config.port = Port

	t.Logf("Waiting for MySQL to accept connections")

	err := c.Wait(Port, MySQLUpWaitTime)

	if err != nil {
		c.Drop()
		t.Fatal("MySQL failed to start.", err)
	}

	t.Log("MySQL is now accepting connections")
	db, err := sql.Open("mysql", config.url())

	if err != nil {
		c.Drop()
		t.Fatal(err)
	}

	return db, c
}
