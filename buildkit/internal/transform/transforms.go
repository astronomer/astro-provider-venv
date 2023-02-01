package transform

import (
	"bytes"
	"fmt"
	"github.com/astronomer/astro-runtime-frontend/internal/dockerfile"
	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"regexp"
	"strings"
	"text/template"
)

const (
	pythonEnvTemplate = `USER root
COPY --link --from=python:{{.PythonVersion}}-slim /usr/local/bin/*{{.PythonMajorMinor}}* /usr/local/bin/
COPY --link --from=python:{{.PythonVersion}}-slim /usr/local/include/python{{.PythonMajorMinor}}* /usr/local/include/python{{.PythonMajorMinor}}
COPY --link --from=python:{{.PythonVersion}}-slim /usr/local/lib/pkgconfig/*{{.PythonMajorMinor}}* /usr/local/lib/pkgconfig/
COPY --link --from=python:{{.PythonVersion}}-slim /usr/local/lib/*{{.PythonMajorMinor}}*.so* /usr/local/lib/
COPY --link --from=python:{{.PythonVersion}}-slim /usr/local/lib/python{{.PythonMajorMinor}} /usr/local/lib/python{{.PythonMajorMinor}}
RUN /sbin/ldconfig /usr/local/lib
# hack for python <= 3.7
RUN ln -s /usr/local/include/python{{.PythonMajorMinor}} /usr/local/include/python{{.PythonMajorMinor}}m
USER astro
`
	virtualEnvTemplate = `RUN mkdir -p /home/astro/.venv/{{.Name}}
{{if .RequirementsFile}}COPY {{.RequirementsFile}} /home/astro/.venv/{{.Name}}/requirements.txt{{end}}
RUN /usr/local/bin/python{{.PythonMajorMinor}} -m venv --system-site-packages /home/astro/.venv/{{.Name}}
ENV ASTRO_PYENV_{{.Name}} /home/astro/.venv/{{.Name}}/bin/python
{{if .RequirementsFile}}RUN --mount=type=cache,target=/home/astro/.cache/pip /home/astro/.venv/{{.Name}}/bin/pip --cache-dir=/home/astro/.cache/pip install -r /home/astro/.venv/{{.Name}}/requirements.txt{{end}}
`
	fromCommand  = "FROM"
	pyenvCommand = "PYENV"
)

var (
	venvNamePattern      = regexp.MustCompile(`[a-zA-Z0-9_-]+`)
	pythonVersionPattern = regexp.MustCompile(`[0-9]+\.[0-9]+(\.[0-9]+)?(-.*)?`)
)

type Transformer struct {
	pythonVersions map[string]struct{}
	virtualEnvs    map[string]struct{}
}

type virtualEnv struct {
	Name             string
	PythonVersion    string
	PythonMajorMinor string
	RequirementsFile string
}

func newTransformer() *Transformer {
	return &Transformer{
		pythonVersions: map[string]struct{}{},
	}
}

// Transform processes the custom dockerfile and extracts the first FROM and converts the rest of the user dockerfile
// into standard Docker commands
func Transform(dockerFile []byte) (*parser.Node, *parser.Node, error) {
	ast, err := dockerfile.Parse(dockerFile)
	if err != nil {
		return nil, nil, err
	}
	transformedAst := &parser.Node{StartLine: -1}
	preamble := &parser.Node{StartLine: -1}
	transformer := newTransformer()
	inPreamble := true
	adjustedLine := 0
	for _, node := range ast.Children {
		switch strings.ToUpper(node.Value) {
		case pyenvCommand:
			pyenvNodes, err := transformer.processPyenv(node)
			if err != nil {
				return nil, nil, err
			}
			lineOffset := 0
			for _, n := range pyenvNodes.Children {
				transformedAst.AddChild(n, n.StartLine+adjustedLine, n.EndLine+adjustedLine)
				lineOffset += (n.EndLine - n.StartLine) + 1
			}
			adjustedLine += lineOffset
		default:
			if inPreamble {
				preamble.AddChild(node, node.StartLine, node.EndLine)
				if strings.ToUpper(node.Value) == fromCommand {
					// finish preamble when we encounter the first FROM
					// TODO: validate image here
					adjustedLine = -node.EndLine
					inPreamble = false
				}
			} else {
				lineDiff := node.EndLine - node.StartLine
				transformedAst.AddChild(node, adjustedLine+1, lineDiff+adjustedLine+1)
			}
		}
	}
	return preamble, transformedAst, nil
}

func (r *Transformer) processPyenv(pyenv *parser.Node) (*parser.Node, error) {
	venv, err := parsePyenvDirective(pyenv.Original)
	if err != nil {
		return nil, err
	}
	newNode := &parser.Node{}
	pythonVersionNode, err := r.addPythonVersion(venv)
	if err != nil {
		return nil, err
	}
	lineOffset := 0
	for _, n := range pythonVersionNode.Children {
		newNode.AddChild(n, n.StartLine, n.EndLine)
		lineOffset = n.EndLine
	}
	venvNode, err := r.addVirtualEnvironment(venv)
	if err != nil {
		return nil, err
	}
	for _, n := range venvNode.Children {
		newNode.AddChild(n, n.StartLine+lineOffset, n.EndLine+lineOffset)
	}
	return newNode, nil
}

func parsePyenvDirective(s string) (*virtualEnv, error) {
	tokens := strings.Split(s, " ")
	if len(tokens) < 3 {
		return nil, fmt.Errorf("invalid PYENV directive: '%s', should be 'PYENV PYTHON_VERSION VENV_NAME [REQS_FILE]'", s)
	}
	env := &virtualEnv{
		PythonVersion: tokens[1],
		Name:          tokens[2],
	}
	if len(tokens) > 3 {
		env.RequirementsFile = tokens[3]
	}
	if err := validateVirtualEnv(env); err != nil {
		return nil, err
	}
	env.PythonMajorMinor = extractPythonMajorMinor(env.PythonVersion)
	return env, nil
}

func validateVirtualEnv(env *virtualEnv) error {
	// validate python version
	if !pythonVersionPattern.MatchString(env.PythonVersion) {
		return fmt.Errorf("invalid python version %s, should match pattern %v", env.PythonVersion, pythonVersionPattern)
	}
	// validate venv name
	if !venvNamePattern.MatchString(env.Name) {
		return fmt.Errorf("invalid virtual env name %s, should match pattern %v", env.Name, venvNamePattern)
	}
	return nil
}

func extractPythonMajorMinor(pythonVersion string) string {
	// we have validated the python version at this point so "safe" to assume at least two fields
	return strings.Join(strings.Split(pythonVersion, ".")[:2], ".")
}

func (r *Transformer) addPythonVersion(venv *virtualEnv) (*parser.Node, error) {
	if _, exists := r.pythonVersions[venv.PythonMajorMinor]; exists {
		return nil, nil
	}
	tpl, err := template.New("pyenv").Parse(pythonEnvTemplate)
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	if err := tpl.Execute(buf, venv); err != nil {
		return nil, err
	}
	parsedNodes, err := parser.Parse(buf)
	if err != nil {
		return nil, err
	}
	return parsedNodes.AST, nil
}

func (r *Transformer) addVirtualEnvironment(venv *virtualEnv) (*parser.Node, error) {
	if _, exists := r.virtualEnvs[venv.Name]; exists {
		return nil, fmt.Errorf("")
	}
	tpl, err := template.New("venv").Parse(virtualEnvTemplate)
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	if err := tpl.Execute(buf, venv); err != nil {
		return nil, err
	}
	parsedNodes, err := parser.Parse(buf)
	if err != nil {
		return nil, err
	}
	return parsedNodes.AST, nil
}

func ensureValidBaseImage(from *parser.Node) error {
	ref, err := reference.ParseNormalizedNamed(from.Value)
	if err != nil {
		return err
	}
	if strings.HasPrefix(ref.Name(), "quay.io/astronomer/astro-runtime") {
		if !strings.HasSuffix(ref.Name(), "-base") {
			from.Value = ref.Name() + "-base"
		}
	}
	return nil
}
