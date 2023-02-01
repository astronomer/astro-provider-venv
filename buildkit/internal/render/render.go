package render

import (
	"bytes"
	"text/template"
)

const dockerfileTemplate = `{{.Preamble}}

SHELL ["/bin/bash", "-o", "pipefail", "-e", "-u", "-x", "-c"]
LABEL maintainer="Astronomer <humans@astronomer.io>"
LABEL io.astronomer.docker=true

COPY packages.txt .
USER root
RUN if [[ -s packages.txt ]]; then \
    apt-get update && cat packages.txt | tr '\r\n' '\n' | xargs apt-get install -y --no-install-recommends \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*; \
  fi
USER astro

{{.Body}}

# Install python packages
USER root
COPY requirements.txt .
RUN if grep -Eqx 'apache-airflow\s*[=~>]{1,2}.*' requirements.txt; then \
    echo >&2 "Do not upgrade by specifying 'apache-airflow' in your requirements.txt, change the base image instead!";  exit 1; \
  fi; \
  pip install --no-cache-dir -r requirements.txt
USER astro

# Copy entire project directory
COPY --chown=astro:astro . .
`

type userCustomisations struct {
	Preamble string
	Body     string
}

func RenderDockerfileTemplate(preamble, body string) ([]byte, error) {
	tpl := template.New("venv")
	venvTpl, err := tpl.Parse(dockerfileTemplate)
	if err != nil {
		return nil, err
	}
	b := &bytes.Buffer{}
	if err := venvTpl.Execute(b, userCustomisations{
		Preamble: preamble,
		Body:     body,
	}); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
