package conex

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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

	// Tart runner doesn't need a Docker client.
	if mn.runnerType == RunnerTart {
		runnerConfig := &RunnerConfig{
			Name:    mn.name,
			Counter: mn.counter,
		}
		mn.runner = NewTartRunner(runnerConfig)

		if mn.pullImages {
			err = mn.tartPull(images)
		}

		if err != nil {
			fmt.Println(err)
			return mn.retcode
		}

		log.Println()
		fmt.Fprintf(os.Stderr, "=== conex: Starting your tests.\n")
		ret := mn.runner.Run(m)
		return ret
	}

	// Ensure DOCKER_API_VERSION is set so go-dockerclient negotiates
	// correctly. Without it, the client defaults to 1.25 which is
	// rejected by modern Docker daemons.
	if os.Getenv("DOCKER_API_VERSION") == "" {
		os.Setenv("DOCKER_API_VERSION", "1.43")
	}

	mn.client, err = docker.NewClientFromEnv()
	if err != nil {
		fmt.Println(err)
		return mn.retcode
	}

	// Ping the Docker server to initialize the client's API version.
	// This prevents a race condition in go-dockerclient when multiple
	// goroutines call methods that trigger checkAPIVersion() concurrently.
	if err := mn.client.Ping(); err != nil {
		fmt.Println("Failed to ping Docker:", err)
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
	fmt.Fprintf(os.Stderr, "=== conex: Starting your tests.\n")

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
	// If image is a Dockerfile, resolve to the built tag.
	resolvedConf := conf
	if isDockerfile(conf.Image) {
		copy := *conf
		copy.Image = dockerfileTag(conf.Image)
		resolvedConf = &copy
	}
	name := mn.boxName(t.Name(), resolvedConf.Image)
	return mn.runner.Box(t, resolvedConf, name)
}

func (mn *manager) pull(images []string) error {
	if len(images) == 0 {
		return nil
	}

	var pullImages []string
	var buildImages []string

	for _, image := range images {
		if isDockerfile(image) {
			buildImages = append(buildImages, image)
		} else {
			pullImages = append(pullImages, image)
		}
	}

	if len(pullImages) > 0 {
		log.Println()
		fmt.Fprintf(os.Stderr, "=== conex: Pulling Images\n")

		l := len(pullImages)
		for i, image := range pullImages {
			fmt.Fprintf(os.Stderr, "--- Pulling %s (%d of %d)\n", image, i+1, l)

			repo, tag := docker.ParseRepositoryTag(image)
			if tag == "" {
				tag = "latest"
			}

			err := mn.client.PullImage(
				docker.PullImageOptions{
					Repository:   repo,
					Tag:          tag,
					OutputStream: os.Stderr,
				},
				docker.AuthConfiguration{},
			)

			if err != nil {
				return err
			}
		}

		fmt.Fprintf(os.Stderr, "=== conex: Pulling Done\n")
		log.Println()
	}

	if len(buildImages) > 0 {
		log.Println()
		fmt.Fprintf(os.Stderr, "=== conex: Building Images\n")

		for i, image := range buildImages {
			tag := dockerfileTag(image)
			fmt.Fprintf(os.Stderr, "--- Building %s as %s (%d of %d)\n", image, tag, i+1, len(buildImages))

			dir := filepath.Dir(image)
			dockerfileName := filepath.Base(image)

			err := mn.client.BuildImage(docker.BuildImageOptions{
				Name:         tag,
				Dockerfile:   dockerfileName,
				ContextDir:   dir,
				OutputStream: os.Stderr,
			})

			if err != nil {
				return fmt.Errorf("build %s: %w", image, err)
			}
		}

		fmt.Fprintf(os.Stderr, "=== conex: Building Done\n")
		log.Println()
	}

	return nil
}

// isDockerfile returns true if the image string looks like a path to a
// Dockerfile rather than a registry image reference.
func isDockerfile(image string) bool {
	base := filepath.Base(image)
	return strings.HasPrefix(base, "Dockerfile")
}

// dockerfileTag generates a conex image tag from a Dockerfile path.
// e.g. "./testdata/Dockerfile.ssh" -> "conex-build:dockerfile-ssh"
func dockerfileTag(path string) string {
	base := filepath.Base(path)
	base = strings.ToLower(base)
	base = strings.ReplaceAll(base, ".", "-")
	return "conex-build:" + base
}

func (mn *manager) ensure(images []string) error {
	if len(images) == 0 {
		return nil
	}

	log.Println() // Print a timestamp, handy to check if something is stack.
	fmt.Fprintf(os.Stderr, "=== conex: Checking for Images\n\n")

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

	fmt.Fprintf(os.Stderr, "\n=== conex: All Images Found.\n")

	return nil
}

func (mn *manager) cleanup() error {
	return nil
}

// tartPull ensures Tart VM images are available locally by pulling them.
func (mn *manager) tartPull(images []string) error {
	if len(images) == 0 {
		return nil
	}

	log.Println()
	fmt.Fprintf(os.Stderr, "=== conex: Pulling Tart Images\n")

	l := len(images)
	for i, image := range images {
		fmt.Fprintf(os.Stderr, "--- Pulling %s (%d of %d)\n", image, i+1, l)
		if _, err := tartCmd("pull", image); err != nil {
			return fmt.Errorf("failed to pull tart image %s: %w", image, err)
		}
	}

	fmt.Fprintf(os.Stderr, "=== conex: Pulling Done\n")
	log.Println()

	return nil
}

func printImg(width int, ref string, index int, total int, img *docker.Image) error {

	fmt.Fprintf(os.Stderr, "--- Found (%d of %d) %-*s %s %10s ago\n",
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
