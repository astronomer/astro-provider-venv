package frontend

import (
	"context"
	"fmt"
	"strings"

	"github.com/EricHripko/buildkit-fdk/pkg/dtp"
	"github.com/astronomer/astro-runtime-frontend/internal/dockerfile"
	"github.com/astronomer/astro-runtime-frontend/internal/render"
	"github.com/astronomer/astro-runtime-frontend/internal/transform"
	"github.com/moby/buildkit/frontend/dockerfile/builder"
	"github.com/moby/buildkit/frontend/gateway/client"
)

const buildArgPrefix = "build-arg:"

func filterByPrefix(opt map[string]string, key string) map[string]string {
	m := map[string]string{}
	for k, v := range opt {
		if strings.HasPrefix(k, key) {
			m[strings.TrimPrefix(k, key)] = v
		}
	}
	return m
}

func Build(ctx context.Context, c client.Client) (*client.Result, error) {
	buildArgs := filterByPrefix(c.BuildOpts().Opts, buildArgPrefix)

	// Process user dockerfile and wrap in standard astro runtime boilerplate
	dockerfileRaw, err := dockerfile.Read(ctx, c)
	if err != nil {
		return nil, err
	}
	preamble, transformedAST, err := transform.Transform(dockerfileRaw, buildArgs)
	if err != nil {
		return nil, err
	}
	if preamble == nil || len(preamble.Children) == 0 {
		return nil, fmt.Errorf("no FROM command in user dockerfile")
	}
	preambleText, err := dockerfile.Print(preamble)
	if err != nil {
		return nil, err
	}
	bodyText, err := dockerfile.Print(transformedAST)
	if err != nil {
		return nil, err
	}
	renderedDockerfile, err := render.RenderDockerfileTemplate(preambleText, bodyText)
	if err != nil {
		return nil, err
	}

	// Pass on our transformed dockerfile
	transformFunc := func(replacementDockerfile []byte) func(dockerfile []byte) ([]byte, error) {
		return func(dockerfile []byte) ([]byte, error) {
			return replacementDockerfile, nil
		}
	}(renderedDockerfile)
	if err := dtp.InjectDockerfileTransform(transformFunc, c); err != nil {
		return nil, err
	}

	// Pass control to the upstream Dockerfile frontend
	return builder.Build(ctx, c)
}
