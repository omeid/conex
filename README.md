# Conex [![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://godoc.org/github.com/omeid/conex)  [![Build Status](https://travis-ci.org/omeid/conex.svg?branch=master)](https://travis-ci.org/omeid/conex)
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



## Drivers Packages

Conex drivers a simple packages following a convention to provide a simple interface
to the underlying service using their native driver/clients so you don't have to think about containers in your tests.

Define an image attribute for your package that users can change and register it with conex.

```go
// Image to use for the box.
var Image = "redis:alpine"

func init() {
  conex.Require(func() string { return Image })
}
```

Then export a box function that returns a client connect to the container and a function to be called when the user is done with the client and thus container.

```go
// Box returns an connect to an echo container based on
// your provided tags.
func Box(t *testing.T, optionally SomeOptions) (your.Client, func()) {
  c := conex.Box(t, Image)

  // Define a function that calls container.Drop when called.
  done := func() {
    err := c.Drop()
    if err != nil {
      t.Fatal(err)
    }
  }

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
