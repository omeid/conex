package conex

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/cli/command"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stringid"
	units "github.com/docker/go-units"
)

// New returns a new conex manager.
func New(retcode int, pullImages bool, images ...string) Manager {
	return &manager{
		retcode:    retcode,
		pullImages: pullImages,
		images:     images,
		counter:    &counter{seqs: make(map[string]int)},
	}
}

type manager struct {
	retcode    int
	pullImages bool

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
		fmt.Println(err)
		return mn.retcode
	}

	mn.images = append(mn.images, images...)

	mn.client, err = docker.NewEnvClient()
	if err != nil {
		fmt.Println(err)
		return mn.retcode
	}

	if mn.pullImages {
		err = mn.pull(images)
	} else {
		err = mn.ensure(images)
	}

	if err != nil {
		fmt.Println(err)
		return mn.retcode
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

func (mn *manager) boxName(test string, image string, params []string) string {
	image = strings.Replace(image, ":", ".", -1)
	image = strings.Replace(image, "/", "_", -1)
	name := fmt.Sprintf("%s-%s-%s", mn.name, test, image)
	name = fmt.Sprintf("%s_%d", name, mn.counter.Count(name))

	return name
}

// Box returns the required container by image name and any tags.
func (mn *manager) Box(t *testing.T, conf *Config) Container {

	name := mn.boxName(t.Name(), conf.Image, conf.Cmd)

	// cname is a simple canonical name that includes the
	// container image name and params.
	cname := conf.Image
	if len(conf.Cmd) != 0 {
		cname = cname + ": " + strings.Join(conf.Cmd, " ")
	}

	logf(t, "creating (%s) as %s", cname, name)

	c, err := mn.client.ContainerCreate(
		context.Background(),
		&dockercontainer.Config{
			Image:      conf.Image,
			Cmd:        strslice.StrSlice(conf.Cmd),
			Env:        conf.Env,
			Hostname:   conf.Hostname,
			Domainname: conf.Domainname,
			User:       conf.User,
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

	return &container{j: cjson, c: mn.client, t: t}
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

func (mn *manager) ensure(images []string) error {
	if len(images) == 0 {
		return nil
	}

	log.Println() // Print a timestamp, handy to check if something is stack.
	fmt.Printf("=== conex: Checking for Images\n\n")

	is := len(images)
	width := maxWidth(images)

	for index, ref := range images {

		img, _, err := mn.client.ImageInspectWithRaw(context.Background(), ref)
		if err != nil {
			return err
		}

		printImg(width, ref, index, is, img)

	}

	fmt.Printf("\n=== conex: All Images Found.\n")

	return nil
}

func (mn *manager) cleanup() error {
	return nil
}

func printImg(width int, ref string, index int, total int, img types.ImageInspect) error {

	createdAt, err := time.Parse(time.RFC3339Nano, img.Created)
	if err != nil {
		return err
	}

	fmt.Printf("--- Found (%d of %d) %-*s %s %10s ago\n",
		index+1,
		total,
		width,
		ref,
		stringid.TruncateID(img.ID),
		units.HumanDuration(time.Now().UTC().Sub(createdAt)),
	)

	return nil
}

func maxWidth(str []string) int {
	max := 0
	for _, s := range str {
		w := len(s)
		if w > max {
			max = w
		}
	}
	return max
}
