package conex

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/cli/command"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
)

// FailReturn is used as status code when conex fails to setup during Run.
// This does not override the return value of testing.M.Run, only when conex
// fails to even testing.M.Run.
var FailReturn = 255

// New returns a new conex manager.
func New(images ...string) Manager {
	return &manager{
		images:  images,
		counter: &counter{seqs: make(map[string]int)},
	}
}

type manager struct {
	name    string
	images  []string
	client  *docker.Client
	counter *counter
}

// Run prepares a docker client, pulls the provided list of images
// and then runs your tests.
func (mn *manager) Run(m *testing.M, images ...string) int {
	// Pull images

	var err error
	mn.name, err = testContainersPrefix()

	if err != nil {
		fmt.Print(err)
		return FailReturn
	}

	mn.images = append(mn.images, images...)

	mn.client, err = docker.NewEnvClient()
	if err != nil {
		fmt.Print(err)
		return FailReturn
	}

	err = mn.pull(images)
	if err != nil {
		fmt.Print(err)
		return FailReturn
	}

	log.Println() // print a timestamp. Helps to see how long tests take on it's own.
	fmt.Printf("=== conex: Starting your tests.\n")
	ret := m.Run()

	err = mn.cleanup()
	if err != nil {
		// TODO: If cleanup fails, tests shouldn't fail, or should they?
		log.Print(err)
	}

	return ret
}

func (mn *manager) boxName(test string, image string, params ...string) string {
	image = strings.Replace(image, ":", ".", -1)
	image = strings.Replace(image, "/", "_", -1)
	return fmt.Sprintf("%s-%s-%s_%d", mn.name, test, image, mn.counter.Count(image, params))
}

// Box returns the required container by image name and any tags.
func (mn *manager) Box(t *testing.T, image string, params ...string) Container {

	name := mn.boxName(t.Name(), image, params...)

	// cname is a simple canonical name that includes the
	// container image name and params.
	cname := image
	if len(params) != 0 {
		cname = cname + ": " + strings.Join(params, " ")
	}

	logf(t, "creating (%s) as %s", cname, name)

	c, err := mn.client.ContainerCreate(
		context.Background(),
		&dockercontainer.Config{
			Image: image,
			Cmd:   strslice.StrSlice(params),
		},
		nil,
		nil,
		name,
	)
	if err != nil {
		fatalf(t, "Failed to create container: %s", err)
	}

	err = mn.client.ContainerStart(context.Background(), c.ID, types.ContainerStartOptions{})
	if err != nil {
		fatalf(t, "Failed to start container: %v", err)
	}

	logf(t, "started (%s) as %s", cname, name)

	cjson, err := mn.client.ContainerInspect(context.Background(), c.ID)

	if err != nil {
		fatalf(t, "Failed to inspect: %v", err)
	}

	return &container{
		client: mn.client,
		c:      cjson,
	}
}

func (mn *manager) pull(images []string) error {
	if len(images) == 0 {
		return nil
	}

	log.Println() // Print a timestamp, handy to check if something is stack.
	fmt.Printf("=== conex: Pulling Images\n")

	l := len(images)

	for i, image := range images {
		fmt.Printf("--- Pulling %s (%d of %d)\n", image, i+1, l)
		output, err := mn.client.ImagePull(context.Background(), image, types.ImagePullOptions{})
		if err != nil {
			return err
		}

		jsonmessage.DisplayJSONMessagesToStream(output, command.NewOutStream(os.Stdout), nil)
	}

	fmt.Printf("=== conex: Pulling Done\n")
	log.Println() // Helps seeing how long the tests take.

	return nil
}

func (mn *manager) cleanup() error {
	return nil
}
