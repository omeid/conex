package conex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	units "github.com/docker/go-units"
	"github.com/moby/go-archive"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/stringid"
)

// runnerType specifies which runner implementation to use.
type RunnerType string

const (
	// RunnerNative runs tests on the host with direct container IP access.
	// This is the default and requires native Docker.
	RunnerNative RunnerType = "native"

	// RunnerDocker runs containers on a shared network, allowing tests to
	// work on systems where container IPs are not accessible from the host
	// (e.g., Docker for Mac, Docker Machine).
	RunnerDocker RunnerType = "docker"

	// RunnerTart runs VMs using Tart virtualization.
	// Container IPs are directly accessible from the host.
	RunnerTart RunnerType = "tart"
)

type managerConfig struct {
	name        string
	runner      RunnerType
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

// OptRunnerType allows setting the RunnerType explicitly.
func OptRunnerType(runner RunnerType) Option {
	return func(conf *managerConfig) { conf.runner = runner }
}

// New creates a new conex manager with the given options.
// Options take precedence over package-level defaults.
func New(options ...Option) Manager {
	conf := &managerConfig{
		runner:      RunnerNative,
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
	runner runner
}

// Run prepares a docker client, pulls the provided list of images
// and then runs your tests.
func (mn *manager) Run(m *testing.M, images ...string) int {
	var err error
	mn.conf.name, err = testContainersPrefix()

	if err != nil {
		return mn.conf.retcode
	}

	allImages := append(append([]string{}, mn.conf.images...), images...)
	if mn.conf.runner == RunnerDocker && mn.conf.goImage != "" {
		allImages = append(allImages, mn.conf.goImage)
	}
	allImages = dedupeImages(allImages)

	if os.Getenv(ConexRunnerEnv) == "1" {
		for i, img := range allImages {
			allImages[i] = DockerfileTag(img)
		}
	}

	mn.conf.images = allImages

	if mn.conf.runner != RunnerTart {
		mn.client, err = client.New(client.FromEnv)
		if err != nil {
			Logf(nil, "conex", "error: %v", err)
			return mn.conf.retcode
		}

		// Ping the Docker server to initialize the client's API version.
		// This prevents a race condition in go-dockerclient when multiple
		// goroutines call methods that trigger checkAPIVersion() concurrently.
		if _, err := mn.client.Ping(context.Background(), client.PingOptions{}); err != nil {
			Logf(nil, "conex", "Failed to ping Docker: %v", err)
			return mn.conf.retcode
		}
	}

	config := &runnerConfig{
		Name:       mn.conf.name,
		PullImages: mn.conf.pullImages,
		Images:     allImages,
		RetCode:    mn.conf.retcode,
		GoImage:    DockerfileTag(mn.conf.goImage),
	}

	// Create the appropriate runner
	switch mn.conf.runner {
	case RunnerTart:
		mn.runner = newTartRunner(config)
	case RunnerDocker:
		mn.runner = NewDockerRunner(mn.client, config)
	default:
		mn.runner = newNativeRunner(mn.client, config)
	}

	pullImages, buildImages := splitImageRefs(allImages)

	if mn.conf.pullImages {
		err = mn.pull(pullImages)
	} else {
		err = mn.ensure(pullImages)
	}
	if err != nil {
		Logf(nil, "conex", "error: %v", err)
		return mn.conf.retcode
	}

	if mn.conf.buildImages {
		err = mn.build(buildImages)
	} else {
		err = mn.ensure(dockerfileTags(buildImages))
	}

	if err != nil {
		Logf(nil, "conex", "error: %v", err)
		return mn.conf.retcode
	}

	Logf(nil, "conex", "Starting your tests.")

	ret := mn.runner.Run(m)

	if mn.conf.runner != RunnerTart {
		err = mn.cleanup()
		if err != nil {
			Logf(nil, "conex", "cleanup error: %v", err)
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

// Box returns the required container by image name and any tags.
func (mn *manager) Box(t testing.TB, conf *Config) Container {
	// If image is a Dockerfile, resolve to the built tag.
	resolvedConf := conf
	if isDockerfile(conf.Image) {
		copy := *conf
		copy.Image = DockerfileTag(conf.Image)
		resolvedConf = &copy
	}
	name := mn.boxName(t.Name(), resolvedConf.Image)
	c := mn.runner.Box(t, resolvedConf, name)
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
	Logf(nil, "", "=== Pulling Images (%d)", l)
	for i, ref := range images {
		if strings.HasPrefix(ref, "conexbuild/") {
			continue
		}
		Logf(nil, "", "--- Pulling Image (%d of %d) %s", i+1, l, ref)
		if err := mn.runner.Pull(context.Background(), ref); err != nil {
			return err
		}
	}
	Logf(nil, "", "=== Pulling Done")
	return nil
}

func (mn *manager) build(images []string) error {
	l := len(images)
	if l == 0 {
		return nil
	}
	Logf(nil, "", "=== Building Images (%d)", l)
	for i, img := range images {
		tag := DockerfileTag(img)
		Logf(nil, "", "--- Building Image (%d of %d) %s as %s", i+1, l, img, tag)
		if err := mn.runner.Build(context.Background(), img, tag); err != nil {
			return err
		}
	}
	Logf(nil, "", "=== Building Done")
	return nil
}

func dockerPull(ctx context.Context, cli client.APIClient, ref string) error {
	reader, err := cli.ImagePull(ctx, ref, client.ImagePullOptions{})
	if err != nil {
		return err
	}
	return printPullProgress(ctx, reader)
}

func dockerBuild(ctx context.Context, cli client.APIClient, img string, tag string) error {
	dir := filepath.Dir(img)
	dockerfileName := filepath.Base(img)
	buildCtx, err := archive.TarWithOptions(dir, &archive.TarOptions{})
	if err != nil {
		return fmt.Errorf("archive %s: %w", img, err)
	}
	res, err := cli.ImageBuild(ctx, buildCtx, client.ImageBuildOptions{
		Tags:       []string{tag},
		Dockerfile: dockerfileName,
		Remove:     true,
	})
	if err != nil {
		_ = buildCtx.Close()
		return fmt.Errorf("build %s: %w", img, err)
	}
	err = printBuildProgress(ctx, res.Body)
	_ = buildCtx.Close()
	if err != nil {
		return fmt.Errorf("build %s: %w", img, err)
	}
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
	Logf(nil, "", "=== Checking for Images (%d)", l)
	width := maxWidth(images)
	for index, ref := range images {
		info, err := mn.runner.Ensure(context.Background(), ref)
		if err != nil {
			return err
		}
		Logf(nil, "", "--- Checked Image (%d of %d) %-*s %s", index+1, l, width, ref, info)
	}
	Logf(nil, "", "=== All Images Found.")
	return nil
}

func dockerEnsure(ctx context.Context, cli client.APIClient, ref string) (string, error) {
	res, err := cli.ImageInspect(ctx, ref)
	if err != nil {
		return "", err
	}
	img := res.InspectResponse
	createdTime, err := time.Parse(time.RFC3339Nano, img.Created)
	if err != nil {
		createdTime, _ = time.Parse(time.RFC3339, img.Created)
	}
	return fmt.Sprintf("%s %10s ago", stringid.TruncateID(img.ID), units.HumanDuration(time.Now().UTC().Sub(createdTime))), nil
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
