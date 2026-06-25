package conex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"testing"

	"github.com/moby/moby/client"

	iruntime "github.com/omeid/conex/internal/runtime"
	"github.com/omeid/conex/internal/runtime/docker"
	"github.com/omeid/conex/internal/runtime/vm"
	"github.com/omeid/conex/log"
	"github.com/omeid/conex/runtime"
)

// RuntimeType specifies which runtime implementation to use.
type RuntimeType string

const (
	// RuntimeNative runs tests on the host with direct container IP access.
	// This is the default and requires native Docker.
	RuntimeNative RuntimeType = "native"

	// RuntimeDocker runs containers on a shared network, allowing tests to
	// work on systems where container IPs are not accessible from the host
	// (e.g., Docker for Mac, Docker Machine).
	RuntimeDocker RuntimeType = "docker"

	// RuntimeVM runs VMs using VM virtualization.
	// Container IPs are directly accessible from the host.
	RuntimeVM RuntimeType = "vm"
)

type managerConfig struct {
	name        string
	runtime     RuntimeType
	retcode     int
	pullImages  bool
	buildImages bool
	goImage     string
	images      []string
}

type Option func(conf *managerConfig)

func OptReturnCode(code int) Option {
	return func(conf *managerConfig) { conf.retcode = code }
}

func OptPullImages(pull bool) Option {
	return func(conf *managerConfig) { conf.pullImages = pull }
}

func OptBuildImages(build bool) Option {
	return func(conf *managerConfig) { conf.buildImages = build }
}

func OptGoImage(image string) Option {
	return func(conf *managerConfig) { conf.goImage = image }
}

func OptRequireImage(image string) Option {
	return func(conf *managerConfig) { conf.images = append(conf.images, image) }
}

// OptRuntimeType allows setting the RuntimeType explicitly.
func OptRuntimeType(runtimeType RuntimeType) Option {
	return func(conf *managerConfig) { conf.runtime = runtimeType }
}

// New creates a new conex manager with the given options.
// Options take precedence over package-level defaults.
func New(options ...Option) Manager {
	conf := &managerConfig{
		runtime:     RuntimeNative,
		pullImages:  PullImages,
		buildImages: BuildImages,
		retcode:     FailReturnCode,
		goImage:     GoImage,
	}

	for _, opt := range options {
		opt(conf)
	}

	return newManager(conf)
}

// newManager is the internal constructor that accepts all options.
func newManager(conf *managerConfig) Manager {
	return &manager{
		conf: conf,
	}
}

type manager struct {
	conf   *managerConfig
	client client.APIClient
	rt     iruntime.Runtime
}

func callerPkg() string {
	for i := 1; ; i++ {
		pc, _, _, ok := stdruntime.Caller(i)
		if !ok {
			break
		}
		name := stdruntime.FuncForPC(pc).Name()
		if before, ok := strings.CutSuffix(name, ".TestMain"); ok {
			return before
		}
	}
	return "unknown"
}

// Run prepares a docker client, pulls the provided list of images
// and then runs your tests.
func (mn *manager) Run(m *testing.M, images ...string) int {
	fmt.Printf("conex %s\n", callerPkg())

	var err error
	mn.conf.name, err = testContainersPrefix()

	if err != nil {
		return mn.conf.retcode
	}

	allImages := append(append([]string{}, mn.conf.images...), images...)
	if mn.conf.runtime == RuntimeDocker && mn.conf.goImage != "" {
		allImages = append(allImages, mn.conf.goImage)
	}
	allImages = dedupeImages(allImages)

	if os.Getenv(docker.ConexRuntimeEnv) == "1" {
		for i, img := range allImages {
			allImages[i] = DockerfileTag(img)
		}
	}

	mn.conf.images = allImages

	if mn.conf.runtime != RuntimeVM {
		mn.client, err = client.New(client.FromEnv)
		if err != nil {
			log.Logf(nil, "conex", "error: %v", err)
			return mn.conf.retcode
		}

		// Ping the Docker server to initialize the client's API version.
		// This prevents a race condition in go-dockerclient when multiple
		// goroutines call methods that trigger checkAPIVersion() concurrently.
		if _, err := mn.client.Ping(context.Background(), client.PingOptions{}); err != nil {
			log.Logf(nil, "conex", "Failed to ping Docker: %v", err)
			return mn.conf.retcode
		}
	}

	config := &iruntime.Config{
		Name:       mn.conf.name,
		PullImages: mn.conf.pullImages,
		Images:     allImages,
		RetCode:    mn.conf.retcode,
		GoImage:    DockerfileTag(mn.conf.goImage),
	}

	// Create the appropriate runtime
	switch mn.conf.runtime {
	case RuntimeVM:
		mn.rt = vm.NewVMRuntime(config)
	case RuntimeDocker:
		mn.rt = docker.NewDockerRuntime(mn.client, config)
	default:
		mn.rt = docker.NewNativeRuntime(mn.client, config)
	}

	pullImages, buildImages := splitImageRefs(allImages)

	if mn.conf.pullImages {
		err = mn.pull(pullImages)
	} else {
		err = mn.ensure(pullImages)
	}
	if err != nil {
		log.Logf(nil, "conex", "error: %v", err)
		return mn.conf.retcode
	}

	if mn.conf.buildImages {
		err = mn.build(buildImages)
	} else {
		err = mn.ensure(dockerfileTags(buildImages))
	}

	if err != nil {
		log.Logf(nil, "conex", "error: %v", err)
		return mn.conf.retcode
	}

	log.Logf(nil, "conex", "Starting your tests.")

	ret := mn.rt.Run(m)

	if mn.conf.runtime != RuntimeVM {
		err = mn.cleanup()
		if err != nil {
			log.Logf(nil, "conex", "cleanup error: %v", err)
		}
	}

	return ret
}

func (mn *manager) boxName(test string, image string) string {
	image = strings.ReplaceAll(image, ":", ".")
	image = strings.ReplaceAll(image, "/", "_")
	name := fmt.Sprintf("%s-%s-%s", mn.conf.name, test, image)

	return name
}

// Box returns a container.
func (mn *manager) Box(t testing.TB, conf *runtime.Config) runtime.Container {
	t.Helper()
	// If image is a Dockerfile, resolve to the built tag.
	resolvedConf := conf
	if isDockerfile(conf.Image) {
		copy := *conf
		copy.Image = DockerfileTag(conf.Image)
		resolvedConf = &copy
	}
	name := mn.boxName(t.Name(), resolvedConf.Image)
	c := mn.rt.Box(t, resolvedConf, name)
	t.Cleanup(func() {
		c.Drop()
	})
	return c
}

func (mn *manager) pull(images []string) error {
	if len(images) == 0 {
		return nil
	}

	l := len(images)
	log.Logf(nil, "", "=== Pulling Images (%d)", l)
	for i, ref := range images {
		if strings.HasPrefix(ref, "conexbuild/") {
			continue
		}
		log.Logf(nil, "", "--- Pulling Image (%d of %d) %s", i+1, l, ref)
		if err := mn.rt.Pull(context.Background(), ref); err != nil {
			return err
		}
	}
	log.Logf(nil, "", "=== Pulling Done")
	return nil
}

func (mn *manager) build(images []string) error {
	l := len(images)
	if l == 0 {
		return nil
	}
	log.Logf(nil, "", "=== Building Images (%d)", l)
	for i, img := range images {
		tag := DockerfileTag(img)
		log.Logf(nil, "", "--- Building Image (%d of %d) %s as %s", i+1, l, img, tag)
		if err := mn.rt.Build(context.Background(), img, tag); err != nil {
			return err
		}
	}
	log.Logf(nil, "", "=== Building Done")
	return nil
}

// isDockerfile returns true if the image string looks like a path to a
// Dockerfile rather than a registry image reference.
func isDockerfile(image string) bool {
	base := filepath.Base(image)
	return strings.HasPrefix(base, "Dockerfile")
}

// DockerfileTag generates a conex image tag from a Dockerfile path.
// e.g. "./testdata/Dockerfile.ssh" -> "conexbuild/dockerfile-ssh"
func DockerfileTag(path string) string {
	if !isDockerfile(path) {
		return path
	}
	base := filepath.Base(path)
	base = strings.ToLower(base)
	base = strings.ReplaceAll(base, ".", "-")
	return "conexbuild/" + base
}

func splitImageRefs(images []string) (pullImages []string, buildImages []string) {
	for _, image := range images {
		if isDockerfile(image) {
			buildImages = append(buildImages, image)
		} else {
			pullImages = append(pullImages, image)
		}
	}
	return pullImages, buildImages
}

func dockerfileTags(images []string) []string {
	tags := make([]string, 0, len(images))
	for _, image := range images {
		tags = append(tags, DockerfileTag(image))
	}
	return tags
}

func dedupeImages(images []string) []string {
	seen := make(map[string]struct{}, len(images))
	uniq := make([]string, 0, len(images))
	for _, image := range images {
		if image == "" {
			continue
		}
		if _, ok := seen[image]; ok {
			continue
		}
		seen[image] = struct{}{}
		uniq = append(uniq, image)
	}
	return uniq
}

func (mn *manager) ensure(images []string) error {
	if len(images) == 0 {
		return nil
	}
	l := len(images)
	log.Logf(nil, "", "=== Checking for Images (%d)", l)
	width := maxWidth(images)
	for index, ref := range images {
		info, err := mn.rt.Ensure(context.Background(), ref)
		if err != nil {
			return err
		}
		log.Logf(nil, "", "--- Checked Image (%d of %d) %-*s %s", index+1, l, width, ref, info)
	}
	log.Logf(nil, "", "=== All Images Found.")
	return nil
}

func (mn *manager) cleanup() error {
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
