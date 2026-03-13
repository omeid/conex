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

// RunnerType specifies which runner implementation to use.
type RunnerType string

const (
	// RunnerNative runs tests on the host with direct container IP access.
	// This is the default and requires native Docker.
	RunnerNative RunnerType = "native"

	// RunnerDocker runs containers on a shared network, allowing tests to
	// work on systems where container IPs are not accessible from the host
	// (e.g., Docker for Mac, Docker Machine).
	RunnerDocker RunnerType = "docker"
)

// New returns a new conex manager.
func New(retcode int, pullImages bool, images ...string) Manager {
	return newManager(retcode, pullImages, RunnerNative, "", images...)
}

// newManager is the internal constructor that accepts all options.
func newManager(retcode int, pullImages bool, runnerType RunnerType, goImage string, images ...string) Manager {
	return &manager{
		retcode:    retcode,
		pullImages: pullImages,
		images:     images,
		counter:    &counter{seqs: make(map[string]int)},
		runnerType: runnerType,
		goImage:    goImage,
	}
}

type manager struct {
	retcode    int
	pullImages bool

	name       string
	images     []string
	client     *docker.Client
	counter    *counter
	runnerType RunnerType
	runner     Runner
	goImage    string
}

// Run prepares a docker client, pulls the provided list of images
// and then runs your tests.
func (mn *manager) Run(m *testing.M, images ...string) int {
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

	// Create the runner configuration
	runnerConfig := &RunnerConfig{
		Client:     mn.client,
		Name:       mn.name,
		PullImages: mn.pullImages,
		Images:     mn.images,
		RetCode:    mn.retcode,
		Counter:    mn.counter,
		GoImage:    mn.goImage,
	}

	// Create the appropriate runner
	switch mn.runnerType {
	case RunnerDocker:
		mn.runner = NewDockerRunner(runnerConfig)
		// Add Go image to the list of images to pull for docker runner
		if mn.goImage != "" {
			images = append(images, mn.goImage)
		}
	default:
		mn.runner = NewNativeRunner(runnerConfig)
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

	ret := mn.runner.Run(m)

	err = mn.cleanup()
	if err != nil {
		// TODO: If cleanup fails, tests shouldn't fail, or should they?
		log.Print(err)
	}

	return ret
}

func (mn *manager) boxName(test string, image string) string {
	image = strings.ReplaceAll(image, ":", ".")
	image = strings.ReplaceAll(image, "/", "_")
	name := fmt.Sprintf("%s-%s-%s", mn.name, test, image)
	name = fmt.Sprintf("%s_%d", name, mn.counter.Count(name))

	return name
}

// Box returns the required container by image name and any tags.
func (mn *manager) Box(t testing.TB, conf *Config) Container {
	name := mn.boxName(t.Name(), conf.Image)
	return mn.runner.Box(t, conf, name)
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
		if tag == "" {
			tag = "latest"
		}

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
