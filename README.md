# Conex [![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://godoc.org/github.com/omeid/conex)  [![Build Status](https://travis-ci.org/omeid/conex.svg?branch=master)](https://travis-ci.org/omeid/conex) [![Go Report Card](https://goreportcard.com/badge/github.com/omeid/conex)](https://goreportcard.com/report/github.com/omeid/conex)
Conex integrates Docker with `testing` package so you can easily run your integration tests.

> Yes, we did hear you like integrations.

## Why?

Integration tests are very good value, they're easy to write and help you catch bugs in a more realistic environment and with most every service and database avaliable as a Docker Container, docker is a great option to run your service dependencies in a clear state. Conex is here to make it simpler. 


## How?

To use conex, we will leverage `TestMain`, this will allow us a starting point to connect to docker, pull all the dependent images and only then run the tests.

Simpley call `conex.Run(m)` where you would run `m.Run()`.
```go
func TestMain(m *testing.M) {
  // If you're planing to use conex.Box directly without
  // using a driver, you can pass your required images
  // after m to conex.Run.
  os.Exit(conex.Run(m))
}
```

In our tests, we will use `driver` packages, these packages register their required image with conex and provide you with a native client and take cares of requesting a container from conex.

Here is an example using redis:

```go
func testPing(t *testing.T) {
  redisDb: = 0
  client, done := redis.Box(t, redisDb)
  defer done() // Return the container.

  // here we can simply use client which is a go-redis
  // client.
}
```

## Example
Here is a complete example using a simple Echo service.

Please note that you can ask for as many containers and different services as you
want, they will all have appropriate names that consist of a uniq id per test, your package path, test name, container, and a serial indicator starting from 0. Not to worry about containers or tests stepping over each other.

```go
package example_test

import (
	"os"
	"testing"

	"github.com/omeid/conex"
	"github.com/omeid/conex/echo"
	echolib "github.com/omeid/echo"
)

func TestMain(m *testing.M) {
	os.Exit(conex.Run(m))
}

func TestEcho(t *testing.T) {
	reverse := true

	e, done := echo.Box(t, reverse)
	defer done()

	say := "hello"
	expect := say
	if reverse {
		expect = echolib.Reverse(say)
	}

	reply, err := e.Say(say)

	if err != nil {
		t.Fatal(err)
	}

	if reply != expect {
		t.Fatalf("\nSaid: %s\nExpected: %s\nGot:      %s\n", say, expect, reply)
	}

}

```

And running tests will yield:

```sh
$ go test -v
2017/04/17 22:13:05 
=== conex: Pulling Images
--- Pulling omeid/echo:http (1 of 1)
http: Pulling from omeid/echo
627beaf3eaaf: Already exists 
8800e3417eb1: Already exists 
b6acb96fee14: Already exists 
66be5afddf19: Already exists 
8ca17cdcfc93: Already exists 
792cf0844f5e: Already exists 
26601152322c: Pull complete 
2cb3c6a6d3ee: Pull complete 
Digest: sha256:f6968275ab031d91a3c37e8a9f65b961b5a3df850a90fe4551ecb4724ab3b0a7
Status: Downloaded newer image for omeid/echo:http
=== conex: Pulling Done
2017/04/17 22:13:38 
2017/04/17 22:13:38 
=== conex: Starting your tests.
=== RUN   TestEcho
--- PASS: TestEcho (0.55s)
	conex.go:11: creating (omeid/echo:http: -reverse) as conex_508151185_test-TestEcho-omeid_echo.http_0
	conex.go:11: started (omeid/echo:http: -reverse) as conex_508151185_test-TestEcho-omeid_echo.http_0
PASS
ok  	test	33.753s
```

## Drivers Packages

Conex drivers are simple packages that follow a convention to provide a simple interface to the underlying service run on the container.
So the user doesn't have to think about containers but the service in their tests.


First, define an image attribute for your package that users can change and register it with conex.

```go
// Image to use for the box.
var Image = "redis:alpine"

func init() {
  conex.Require(func() string { return Image })
}
```

Then export a box function that returns a client connect to the container and return that with the done func.

```go
// Box returns an connect to an echo container based on
// your provided tags.
func Box(t *testing.T, optionally SomeOptions) (your.Client, func()) {
  c, done := conex.Box(t, Image)

  opt := &your.Options{
    Addr: c.Address(),
    magic: optionally.SomeMagic,
  }

  client := redis.NewClient(opt)

  return client, done
}

```

### Is it good?
Yes.

### LICENSE
  [MIT](LICENSE).
