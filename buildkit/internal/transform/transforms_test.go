package transform

import (
	"github.com/astronomer/astro-runtime-frontend/internal/dockerfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

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
FROM ${baseimage}
`
	expectedDockerfile := `USER root
COPY --link --from=python:3.8-slim /usr/local/bin/*3.8* /usr/local/bin/
COPY --link --from=python:3.8-slim /usr/local/include/python3.8* /usr/local/include/python3.8
COPY --link --from=python:3.8-slim /usr/local/lib/pkgconfig/*3.8* /usr/local/lib/pkgconfig/
COPY --link --from=python:3.8-slim /usr/local/lib/*3.8*.so* /usr/local/lib/
COPY --link --from=python:3.8-slim /usr/local/lib/python3.8 /usr/local/lib/python3.8
RUN /sbin/ldconfig /usr/local/lib

RUN ln -s /usr/local/include/python3.8 /usr/local/include/python3.8m
USER astro
RUN mkdir -p /home/astro/.venv/venv1
COPY reqs/venv1.txt /home/astro/.venv/venv1/requirements.txt
RUN /usr/local/bin/python3.8 -m venv --system-site-packages /home/astro/.venv/venv1
ENV ASTRO_PYENV_venv1 /home/astro/.venv/venv1/bin/python
RUN --mount=type=cache,target=/home/astro/.cache/pip /home/astro/.venv/venv1/bin/pip --cache-dir=/home/astro/.cache/pip install -r /home/astro/.venv/venv1/requirements.txt
COPY foo bar
USER root
COPY --link --from=python:3.10-slim /usr/local/bin/*3.10* /usr/local/bin/
COPY --link --from=python:3.10-slim /usr/local/include/python3.10* /usr/local/include/python3.10
COPY --link --from=python:3.10-slim /usr/local/lib/pkgconfig/*3.10* /usr/local/lib/pkgconfig/
COPY --link --from=python:3.10-slim /usr/local/lib/*3.10*.so* /usr/local/lib/
COPY --link --from=python:3.10-slim /usr/local/lib/python3.10 /usr/local/lib/python3.10
RUN /sbin/ldconfig /usr/local/lib

RUN ln -s /usr/local/include/python3.10 /usr/local/include/python3.10m
USER astro
RUN mkdir -p /home/astro/.venv/venv2

RUN /usr/local/bin/python3.10 -m venv --system-site-packages /home/astro/.venv/venv2
ENV ASTRO_PYENV_venv2 /home/astro/.venv/venv2/bin/python
RUN mkdir /tmp/bar
`
	preamble, body, err := Transform([]byte(testDockerfile))
	require.NoError(t, err)
	assert.NotNil(t, preamble)
	assert.NotNil(t, body)
	bodyText, err := dockerfile.Print(body)
	assert.Equal(t, expectedDockerfile, bodyText)
	preambleText, err := dockerfile.Print(preamble)
	assert.Equal(t, expectedPreamble, preambleText)
}
