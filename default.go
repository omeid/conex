package conex

import (
	"testing"
)

var std Manager

var requiredImages = []func() string{}

// Require adds the image name returned by the provided functions
// to the list of images pull by the default Manager when Run is
// called. Used by driver packages, see conex/redis, conex/rethink.
func Require(images ...func() string) {
	requiredImages = append(requiredImages, images...)
}

// Run prepares a docker client, pulls the provided list of images
// and then runs your tests.
func Run(m *testing.M, images ...string) int {

	for _, i := range requiredImages {
		images = append(images, i())
	}

	std = New(images...)

	return std.Run(m, images...)
}

// Box creates a new container using the provided image and passes
// your parameters.
func Box(t *testing.T, conf *Config) Container {
	if std == nil {
		panic("You must call conex.Run first. Use TestMain.")
	}

	return std.Box(t, conf)
}
