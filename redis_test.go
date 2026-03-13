package conex_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis"
	"github.com/omeid/conex"
)

var redisImage = "redis:alpine"

func init() {
	conex.Require(func() string { return redisImage })
}

func TestRedis1(t *testing.T) { t.Parallel(); testPing(t) }
func TestRedis2(t *testing.T) { t.Parallel(); testPing(t) }
func TestRedis3(t *testing.T) { t.Parallel(); testPing(t) }
func TestRedis4(t *testing.T) { t.Parallel(); testPing(t) }

func testPing(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image:  redisImage,
		Expose: []string{"6379"},
	})
	defer c.Drop()

	// Wait for Redis to be ready
	t.Log("Waiting for Redis to accept connections")
	err := c.Wait("6379", 30*time.Second)
	if err != nil {
		t.Fatal("Redis failed to start:", err)
	}
	t.Log("Redis is now accepting connections")

	addr := fmt.Sprintf("%s:6379", c.Address())
	client := redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   0,
	})
	defer client.Close()

	cases := []string{
		"hello",
		"hi",
	}

	for _, say := range cases {
		reply, err := client.Echo(say).Result()
		if err != nil {
			t.Fatal(err)
		}

		if reply != say {
			t.Fatalf("\nExpected: %s\nGot:      %s\n", say, reply)
		}
	}
}
