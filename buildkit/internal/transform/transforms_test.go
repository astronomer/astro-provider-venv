package transform

import (
	"fmt"
	"testing"

	"github.com/astronomer/astro-runtime-frontend/internal/dockerfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func AssertDockerfileTransform(t *testing.T, input, expectedPreamble, expectedDockerfile string) {
	t.Helper()
	AssertDockerfileTransformWithBuildArgs(t, input, map[string]string{}, expectedPreamble, expectedDockerfile)
}

func AssertDockerfileTransformWithBuildArgs(t *testing.T, input string, buildArgs map[string]string, expectedPreamble, expectedDockerfile string) {
	t.Helper()

	preamble, body, err := Transform([]byte(input), buildArgs)
	require.NoError(t, err)
	assert.NotNil(t, preamble)
	assert.NotNil(t, body)
	bodyText, err := dockerfile.Print(body)
	require.NoError(t, err)
	assert.Equal(t, expectedDockerfile, bodyText)
	preambleText, err := dockerfile.Print(preamble)
	require.NoError(t, err)
	assert.Equal(t, expectedPreamble, preambleText)
}

func TestTransformPyenvs(t *testing.T) {
	testDockerfile := `# syntax=astronomer/astro-runtime
ARG baseimage
FROM ${baseimage}
PYENV 3.8 venv1 reqs/venv1.txt
COPY foo bar
PYENV 3.10 venv2
RUN mkdir /tmp/bar
`
	expectedPreamble := `
ARG baseimage
FROM quay.io/astronomer/astro-runtime:13.0.0-base
`
	expectedDockerfile := `USER root
COPY --link --from=python:3.8-slim-bookworm /usr/local/bin/*3.8* /usr/local/bin/
COPY --link --from=python:3.8-slim-bookworm /usr/local/include/python3.8* /usr/local/include/python3.8
COPY --link --from=python:3.8-slim-bookworm /usr/local/lib/pkgconfig/*3.8* /usr/local/lib/pkgconfig/
COPY --link --from=python:3.8-slim-bookworm /usr/local/lib/*3.8*.so* /usr/local/lib/
COPY --link --from=python:3.8-slim-bookworm /usr/local/lib/python3.8 /usr/local/lib/python3.8
RUN /sbin/ldconfig /usr/local/lib

USER astro
RUN mkdir -p /home/astro/.cache/pip /home/astro/.venv/venv1
COPY --chown=50000:0 reqs/venv1.txt /home/astro/.venv/venv1/requirements.txt
RUN /usr/local/bin/python3.8 -m venv /home/astro/.venv/venv1
ENV ASTRO_PYENV_venv1 /home/astro/.venv/venv1/bin/python
RUN --mount=type=cache,uid=50000,gid=0,target=/home/astro/.cache/pip /home/astro/.venv/venv1/bin/pip --cache-dir=/home/astro/.cache/pip install -r /home/astro/.venv/venv1/requirements.txt
COPY foo bar
USER root
COPY --link --from=python:3.10-slim-bookworm /usr/local/bin/*3.10* /usr/local/bin/
COPY --link --from=python:3.10-slim-bookworm /usr/local/include/python3.10* /usr/local/include/python3.10
COPY --link --from=python:3.10-slim-bookworm /usr/local/lib/pkgconfig/*3.10* /usr/local/lib/pkgconfig/
COPY --link --from=python:3.10-slim-bookworm /usr/local/lib/*3.10*.so* /usr/local/lib/
COPY --link --from=python:3.10-slim-bookworm /usr/local/lib/python3.10 /usr/local/lib/python3.10
RUN /sbin/ldconfig /usr/local/lib

USER astro
RUN mkdir -p /home/astro/.cache/pip /home/astro/.venv/venv2

RUN /usr/local/bin/python3.10 -m venv /home/astro/.venv/venv2
ENV ASTRO_PYENV_venv2 /home/astro/.venv/venv2/bin/python
RUN mkdir /tmp/bar
`

	AssertDockerfileTransformWithBuildArgs(t, testDockerfile, map[string]string{"baseimage": "quay.io/astronomer/astro-runtime:13.0.0"}, expectedPreamble, expectedDockerfile)
}

func TestPython3_8(t *testing.T) {
	testDockerfile := `# syntax=astronomer/astro-runtime
ARG baseimage
FROM ${baseimage}
PYENV 3.7 venv1 reqs/venv1.txt
COPY foo bar
`
	expectedPreamble := `
ARG baseimage
FROM ${baseimage}
`
	expectedDockerfile := `USER root
COPY --link --from=python:3.7-slim-bookworm /usr/local/bin/*3.7* /usr/local/bin/
COPY --link --from=python:3.7-slim-bookworm /usr/local/include/python3.7* /usr/local/include/python3.7
COPY --link --from=python:3.7-slim-bookworm /usr/local/lib/pkgconfig/*3.7* /usr/local/lib/pkgconfig/
COPY --link --from=python:3.7-slim-bookworm /usr/local/lib/*3.7*.so* /usr/local/lib/
COPY --link --from=python:3.7-slim-bookworm /usr/local/lib/python3.7 /usr/local/lib/python3.7
RUN /sbin/ldconfig /usr/local/lib
RUN ln -s /usr/local/include/python3.7 /usr/local/include/python3.7m

USER astro
RUN mkdir -p /home/astro/.cache/pip /home/astro/.venv/venv1
COPY --chown=50000:0 reqs/venv1.txt /home/astro/.venv/venv1/requirements.txt
RUN /usr/local/bin/python3.7 -m venv /home/astro/.venv/venv1
ENV ASTRO_PYENV_venv1 /home/astro/.venv/venv1/bin/python
RUN --mount=type=cache,uid=50000,gid=0,target=/home/astro/.cache/pip /home/astro/.venv/venv1/bin/pip --cache-dir=/home/astro/.cache/pip install -r /home/astro/.venv/venv1/requirements.txt
COPY foo bar
`

	AssertDockerfileTransform(t, testDockerfile, expectedPreamble, expectedDockerfile)
}

func TestTransformsBuildArgsFromDockerfile(t *testing.T) {
	testDockerfile := `# syntax=astronomer/astro-runtime
ARG ver=7.0.0
FROM quay.io/astronomer/astro-runtime:${ver}
`
	expectedPreamble := `
ARG ver=7.0.0
FROM quay.io/astronomer/astro-runtime:7.0.0-base
`

	AssertDockerfileTransform(t, testDockerfile, expectedPreamble, "")
}

func TestTransformsBuildArgsProvided(t *testing.T) {
	testDockerfile := `# syntax=astronomer/astro-runtime
ARG ver=7.0.0
FROM quay.io/astronomer/astro-runtime:${ver}
`
	expectedPreamble := `
ARG ver=7.0.0
FROM quay.io/astronomer/astro-runtime:7.4.0-base
`

	AssertDockerfileTransformWithBuildArgs(t, testDockerfile, map[string]string{"ver": "7.4.0"}, expectedPreamble, "")
}

func TestTransformsNonRuntimeLeftAlone(t *testing.T) {
	testDockerfile := `# syntax=astronomer/astro-runtime
FROM astro-runtime:7.0.0
`
	expectedPreamble := `
FROM astro-runtime:7.0.0
`

	preamble, body, err := Transform([]byte(testDockerfile), map[string]string{"ver": "7.4.0"})
	require.NoError(t, err)
	assert.NotNil(t, preamble)
	assert.NotNil(t, body)
	bodyText, err := dockerfile.Print(body)
	assert.Equal(t, "", bodyText)
	preambleText, err := dockerfile.Print(preamble)
	assert.Equal(t, expectedPreamble, preambleText)
}

func TestOnBuild(t *testing.T) {
	testDockerfile := `# syntax=astronomer/astro-runtime
FROM quay.io/astronomer/astro-runtime:7.0.0
ONBUILD RUN echo "hi!"
`

	expectedDockerfile := `ONBUILD RUN echo "hi!"
`

	preamble, body, err := Transform([]byte(testDockerfile), nil)
	require.NoError(t, err)
	assert.NotNil(t, preamble)
	assert.NotNil(t, body)
	bodyText, err := dockerfile.Print(body)
	assert.Equal(t, expectedDockerfile, bodyText)
}

func TestMultipleVenvOfSameVersion(t *testing.T) {
	testDockerfile := `# syntax=astronomer/astro-runtime
FROM quay.io/astronomer/astro-runtime:9.11.0

PYENV 3.11 one
PYENV 3.11 two
`

	expectedPreamble := `
FROM quay.io/astronomer/astro-runtime:9.11.0-base
`

	// Since this is two versions of the same python, we only need to copy it once
	expectedDockerfile := `USER root
COPY --link --from=python:3.11-slim-bullseye /usr/local/bin/*3.11* /usr/local/bin/
COPY --link --from=python:3.11-slim-bullseye /usr/local/include/python3.11* /usr/local/include/python3.11
COPY --link --from=python:3.11-slim-bullseye /usr/local/lib/pkgconfig/*3.11* /usr/local/lib/pkgconfig/
COPY --link --from=python:3.11-slim-bullseye /usr/local/lib/*3.11*.so* /usr/local/lib/
COPY --link --from=python:3.11-slim-bullseye /usr/local/lib/python3.11 /usr/local/lib/python3.11
RUN /sbin/ldconfig /usr/local/lib

USER astro
RUN mkdir -p /home/astro/.cache/pip /home/astro/.venv/one

RUN /usr/local/bin/python3.11 -m venv /home/astro/.venv/one
ENV ASTRO_PYENV_one /home/astro/.venv/one/bin/python
RUN mkdir -p /home/astro/.cache/pip /home/astro/.venv/two

RUN /usr/local/bin/python3.11 -m venv /home/astro/.venv/two
ENV ASTRO_PYENV_two /home/astro/.venv/two/bin/python
`

	AssertDockerfileTransform(t, testDockerfile, expectedPreamble, expectedDockerfile)
}

func TestImageFlavour(t *testing.T) {
	transformer := newTransformer(nil)

	cases := []struct {
		version  string
		expected string
	}{
		{"9.0.1", "slim-bullseye"},
		{"11.12.0", "slim-bullseye"},
		// These versions were mistakenly released based on bookworm
		{"11.13.0", "slim-bookworm"},
		{"11.14.0", "slim-bookworm"},
		{"11.15.0", "slim-bullseye"},
		{"11.15.1", "slim-bullseye"},
		{"11.17.0", "slim-bullseye"},
		// And then this onwards is bullseye properly
		{"12.0.0", "slim-bookworm"},
	}

	for _, tt := range cases {
		t.Run(fmt.Sprintf("version=%s", tt.version), func(t *testing.T) {
			transformer.runtimeImageVersion = tt.version
			got, err := transformer.getImageFlavour()
			if assert.NoError(t, err) {
				assert.Equal(t, got, tt.expected)
			}
		})
	}
}
