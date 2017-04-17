package redis

import (
	"fmt"
	"testing"

	"github.com/go-redis/redis"
	"github.com/omeid/conex"
)

var (
	// Image to use for the box.
	Image = "redis:alpine"
	// Port used for connect to redis.
	Port = "6379"
)

func init() {
	conex.Require(func() string { return Image })
}

// Box returns an echo client connect to an echo container based on
// your provided tags.
func Box(t *testing.T, db int) (*redis.Client, func()) {
	c := conex.Box(t, Image)

	done := func() {
		err := c.Drop()
		if err != nil {
			t.Fatal(err)
		}
	}

	addr := fmt.Sprintf("%s:%s", c.Address(), Port)
	opt := &redis.Options{
		Addr: addr,
		DB:   db,
	}

	client := redis.NewClient(opt)

	return client, done
}
