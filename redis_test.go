package conex_test

import (
	"testing"

	"github.com/conex/redis"
)

func TestRedis1(t *testing.T) { t.Parallel(); testPing(t) }
func TestRedis2(t *testing.T) { t.Parallel(); testPing(t) }
func TestRedis3(t *testing.T) { t.Parallel(); testPing(t) }
func TestRedis4(t *testing.T) { t.Parallel(); testPing(t) }

func testPing(t *testing.T) {

	db := 0

	r, c := redis.Box(t, db)
	defer c.Drop()

	cases := []string{
		"hello",
		"hi",
	}

	for _, say := range cases {

		reply, err := r.Echo(say).Result()

		if err != nil {
			t.Fatal(err)
		}

		if reply != say {
			t.Fatalf("\nExpected: %s\nGot:      %s\n", say, reply)
		}

	}

}
