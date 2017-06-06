package redis

import (
	"fmt"
	"testing"

	"github.com/go-redis/redis"
	"github.com/omeid/conex"
)

func init() {
	fmt.Println("!!! WARNING !!!")
	fmt.Println("github.com/omeid/conex/redis has moved to  github.com/conex/redis")
	fmt.Println("This package will be removed soon.")
}

var (
	// Image to use for the box.
	Image = "redis:alpine"
	// Port used for connect to redis.
	Port = "6379"
)

func init() {
	conex.Require(func() string { return Image })
}

// Box returns a redis.Client and the container running the redis
// server. It calls t.Fatal on errors.
func Box(t testing.TB, db int) (*redis.Client, conex.Container) {
	c := conex.Box(t, &conex.Config{
		Image: Image,
	})

	addr := fmt.Sprintf("%s:%s", c.Address(), Port)
	opt := &redis.Options{
		Addr: addr,
		DB:   db,
	}

	client := redis.NewClient(opt)

	return client, c
}
