package conex

import (
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/pkg/stringid"
	units "github.com/docker/go-units"
	docker "github.com/fsouza/go-dockerclient"
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
		return mn.retcode
	}

	mn.images = append(mn.images, images...)

	mn.client, err = docker.NewClientFromEnv()
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
func (mn *manager) Box(t testing.TB, conf *Config) Container {

	name := mn.boxName(t.Name(), conf.Image, conf.Cmd)

	// cname is a simple canonical name that includes the
	// container image name and params.
	cname := conf.Image
	if len(conf.Cmd) != 0 {
		cname = cname + ": " + strings.Join(conf.Cmd, " ")
	}

	logf(t, "creating (%s) as %s", cname, name)

	exposedPorts := make(map[docker.Port]struct{})
	portBindings := make(map[docker.Port][]docker.PortBinding)
	for _, port := range conf.Expose {
		bindings := strings.Split(port, ":")

		if len(bindings) == 1 {
			exposedPorts[docker.Port(port)] = struct{}{}
			continue
		}

		dockerPort := docker.Port(bindings[1])
		host := docker.PortBinding{
			HostIP:   "0.0.0.0",
			HostPort: bindings[0],
		}

		exposedPorts[dockerPort] = struct{}{}
		portBindings[dockerPort] = []docker.PortBinding{host}
	}

	c, err := mn.client.CreateContainer(
		docker.CreateContainerOptions{
			Name: name,
			Config: &docker.Config{
				Image:        conf.Image,
				Cmd:          conf.Cmd,
				Env:          conf.Env,
				Hostname:     conf.Hostname,
				Domainname:   conf.Domainname,
				User:         conf.User,
				Tty:          true,
				ExposedPorts: exposedPorts,
			},
			HostConfig: &docker.HostConfig{
				PortBindings: portBindings,
			},
		},
	)
	if err != nil {
		fatalf(t, "Failed to create container: %s", err)
	}

	err = mn.client.StartContainer(c.ID, nil)
	if err != nil {
		fatalf(t, "Failed to start container: %v", err)
	}

	logf(t, "started (%s) as %s", cname, name)

	cjson, err := mn.client.InspectContainer(c.ID)

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

		repo, tag := docker.ParseRepositoryTag(image)

		err := mn.client.PullImage(
			docker.PullImageOptions{
				Repository:   repo,
				Tag:          tag,
				OutputStream: os.Stdout,
			},
			docker.AuthConfiguration{},
		)

		if err != nil {
			return err
		}

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

		img, err := mn.client.InspectImage(ref)
		if err != nil {
			return err
		}

		err = printImg(width, ref, index, is, img)
		if err != nil {
			return err
		}

	}

	fmt.Printf("\n=== conex: All Images Found.\n")

	return nil
}

func (mn *manager) cleanup() error {
	return nil
}

func printImg(width int, ref string, index int, total int, img *docker.Image) error {

	fmt.Printf("--- Found (%d of %d) %-*s %s %10s ago\n",
		index+1,
		total,
		width,
		ref,
		stringid.TruncateID(img.ID),
		units.HumanDuration(time.Now().UTC().Sub(img.Created)),
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
