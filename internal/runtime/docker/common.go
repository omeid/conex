package docker

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	units "github.com/docker/go-units"
	"github.com/moby/go-archive"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/stringid"
)

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
