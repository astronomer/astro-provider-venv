package transform

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/astronomer/astro-runtime-frontend/internal/dockerfile"
	"github.com/coreos/go-semver/semver"
	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/moby/buildkit/frontend/dockerfile/shell"
)

const (
	pythonEnvTemplate = `USER root
COPY --link --from=python:{{.PythonVersion}}-{{.PythonFlavour}} /usr/local/bin/*{{.PythonMajorMinor}}* /usr/local/bin/
COPY --link --from=python:{{.PythonVersion}}-{{.PythonFlavour}} /usr/local/include/python{{.PythonMajorMinor}}* /usr/local/include/python{{.PythonMajorMinor}}
COPY --link --from=python:{{.PythonVersion}}-{{.PythonFlavour}} /usr/local/lib/pkgconfig/*{{.PythonMajorMinor}}* /usr/local/lib/pkgconfig/
COPY --link --from=python:{{.PythonVersion}}-{{.PythonFlavour}} /usr/local/lib/*{{.PythonMajorMinor}}*.so* /usr/local/lib/
COPY --link --from=python:{{.PythonVersion}}-{{.PythonFlavour}} /usr/local/lib/python{{.PythonMajorMinor}} /usr/local/lib/python{{.PythonMajorMinor}}
RUN /sbin/ldconfig /usr/local/lib
{{ if .Py37OrOlder -}}
RUN ln -s /usr/local/include/python{{.PythonMajorMinor}} /usr/local/include/python{{.PythonMajorMinor}}m
{{ end }}
USER astro
`
	virtualEnvTemplate = `RUN mkdir -p /home/astro/.cache/pip /home/astro/.venv/{{.Name}}
{{if .RequirementsFile}}COPY --chown={{.AstroUid}}:0 {{.RequirementsFile}} /home/astro/.venv/{{.Name}}/requirements.txt{{end}}
RUN /usr/local/bin/python{{.PythonMajorMinor}} -m venv /home/astro/.venv/{{.Name}}
ENV ASTRO_PYENV_{{.Name}} /home/astro/.venv/{{.Name}}/bin/python
{{if .RequirementsFile}}RUN --mount=type=cache,uid={{.AstroUid}},gid=0,target=/home/astro/.cache/pip /home/astro/.venv/{{.Name}}/bin/pip --cache-dir=/home/astro/.cache/pip install -r /home/astro/.venv/{{.Name}}/requirements.txt{{end}}
`
	fromCommand       = "FROM"
	argCommand        = "ARG"
	pyenvCommand      = "PYENV"
	astroRuntimeImage = "quay.io/astronomer/astro-runtime"

	// Sadly for the `RUN --mount,uid=$uid` we need to use a numeric ID.
	defaultAstroUid = 50000
)

var runtimeImageToFlavour = []struct {
	*semver.Version
	Flavour string
}{
	{semver.New("11.13.0"), "slim-bullseye"}, // < 11.13.0 is on slim-bullseye
	{semver.New("11.14.1"), "slim-bookworm"}, // >= 11.13.0, < 11.14.1 is on slim-bookworm
	{semver.New("12.0.0"), "slim-bullseye"},  // >=11.14.1, < 12.0.0 is on slub.bullseye
	{nil, "slim-bookworm"},                   // Everything after uses slim-bookworm.
}

var (
	venvNamePattern      = regexp.MustCompile(`[a-zA-Z0-9_-]+`)
	pythonVersionPattern = regexp.MustCompile(`[0-9]+\.[0-9]+(\.[0-9]+)?(-.*)?`)
	v3_8                 = *semver.New("3.8.0")
)

type Transformer struct {
	buildArgs      map[string]string
	pythonVersions map[string]struct{}
	virtualEnvs    map[string]struct{}

	// The version of the astro Runtime image we are based upon, if we can determine it
	runtimeImageVersion string
}

type virtualEnv struct {
	Name             string
	PythonVersion    string
	PythonFlavour    string
	PythonMajorMinor string
	RequirementsFile string
	Py37OrOlder      bool
}

func newTransformer(buildArgs map[string]string) *Transformer {
	return &Transformer{
		buildArgs:      buildArgs,
		pythonVersions: map[string]struct{}{},
	}
}

// Transform processes the custom dockerfile and extracts the first FROM and converts the rest of the user dockerfile
// into standard Docker commands
func Transform(dockerFile []byte, buildArgs map[string]string) (*parser.Node, *parser.Node, error) {
	return newTransformer(buildArgs).Transform(dockerFile)
}

func (t *Transformer) parsePreamble(tokens chan *parser.Node) (*parser.Node, int, error) {
	preamble := &parser.Node{StartLine: -1}
	adjustedLine := 0

tokens_loop:
	for node := range tokens {
		preamble.AddChild(node, node.StartLine, node.EndLine)
		switch strings.ToUpper(node.Value) {
		case argCommand:
			if err := t.processArg(node); err != nil {
				return nil, 0, err
			}
		case fromCommand:
			// finish preamble when we encounter the first FROM

			// Try to use the astro-runtime base (non-onbuild) image. If we have any error, "fail safe"
			// and leave it unmodified.
			_ = t.ensureValidBaseImage(node)
			adjustedLine = -node.EndLine
			break tokens_loop
		case pyenvCommand:
			return nil, 0, fmt.Errorf("%s cannot appear before the first FROM", pyenvCommand)
		}
	}
	return preamble, adjustedLine, nil
}

func (t *Transformer) parseMain(tokens chan *parser.Node, lineNumAdjustment int) (*parser.Node, error) {
	transformedAst := &parser.Node{StartLine: -1}

	for node := range tokens {
		switch strings.ToUpper(node.Value) {
		case pyenvCommand:
			pyenvNodes, err := t.processPyenv(node)
			if err != nil {
				return nil, err
			}
			lineOffset := 0
			for _, n := range pyenvNodes.Children {
				transformedAst.AddChild(n, n.StartLine+lineNumAdjustment, n.EndLine+lineNumAdjustment)
				lineOffset += (n.EndLine - n.StartLine) + 1
			}
			lineNumAdjustment += lineOffset
		default:
			lineDiff := node.EndLine - node.StartLine
			transformedAst.AddChild(node, lineNumAdjustment+1, lineDiff+lineNumAdjustment+1)
		}
	}
	return transformedAst, nil
}

func (t *Transformer) Transform(dockerFile []byte) (*parser.Node, *parser.Node, error) {
	ast, err := dockerfile.Parse(dockerFile)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	// tokenStream is an "iterator" over the AST nodes in the Dockerfile
	tokenStream := make(chan *parser.Node)
	go func(ctx context.Context) {
	tokens_loop:
		for _, node := range ast.Children {
			select {
			case <-ctx.Done():
				break tokens_loop
			case tokenStream <- node:
				continue
			}
		}
		close(tokenStream)
	}(ctx)
	defer cancel()

	preamble, adjustedLine, err := t.parsePreamble(tokenStream)
	if err != nil {
		return nil, nil, err
	}
	transformedAst, err := t.parseMain(tokenStream, adjustedLine)

	return preamble, transformedAst, err
}

func (t *Transformer) processArg(node *parser.Node) error {
	inst, err := instructions.ParseInstruction(node)
	if err != nil {
		return err
	}
	if n, ok := inst.(*instructions.ArgCommand); ok {
		for _, pair := range n.Args {
			if pair.Value == nil {
				continue
			}
			if _, exists := t.buildArgs[pair.Key]; !exists {
				t.buildArgs[pair.Key] = *pair.Value
			}
		}
	} else {
		return fmt.Errorf("not a ARG instruction %q", node.Original)
	}

	return nil
}

func (t *Transformer) processPyenv(pyenv *parser.Node) (*parser.Node, error) {
	venv, err := t.parsePyenvDirective(pyenv.Original)
	if err != nil {
		return nil, err
	}
	newNode := &parser.Node{}
	pythonVersionNode, err := t.addPythonVersion(venv)
	if err != nil {
		return nil, err
	}
	lineOffset := 0
	if pythonVersionNode != nil {
		for _, n := range pythonVersionNode.Children {
			newNode.AddChild(n, n.StartLine, n.EndLine)
			lineOffset = n.EndLine
		}
	}
	venvNode, err := t.addVirtualEnvironment(venv)
	if err != nil {
		return nil, err
	}
	for _, n := range venvNode.Children {
		newNode.AddChild(n, n.StartLine+lineOffset, n.EndLine+lineOffset)
	}
	return newNode, nil
}

func (t *Transformer) getImageFlavour() (string, error) {
	// If we are ever unsure, return the latest flavour
	candidate := runtimeImageToFlavour[len(runtimeImageToFlavour)-1].Flavour

	if t.runtimeImageVersion == "" {
		// No runtime version we could find, return whatever the current default version is.
		return candidate, nil
	}
	parts := strings.SplitN(t.runtimeImageVersion, "-", 1)
	ver, err := semver.NewVersion(parts[0])
	if err != nil {
		// If we couldn't parse the version, return a default, don't fail!
		return candidate, err
	}

	// Look through all versions rules until we find one that doesn't match. At
	// that point we know the previous answer we got is the one we need to use
	for _, cond := range runtimeImageToFlavour {
		candidate = cond.Flavour
		if cond.Version != nil && ver.LessThan(*cond.Version) {
			break
		}
	}
	return candidate, nil
}

func (t *Transformer) parsePyenvDirective(s string) (*virtualEnv, error) {
	tokens := strings.Split(s, " ")
	if len(tokens) < 3 {
		return nil, fmt.Errorf("invalid PYENV directive: '%s', should be 'PYENV PYTHON_VERSION VENV_NAME [REQS_FILE]'", s)
	}

	// TODO: Work out how to show warnings to user via docker build backend!
	// For now don't ever fail unless there is no flavour
	flavour, err := t.getImageFlavour()
	if flavour == "" && err != nil {
		return nil, err
	}
	env := &virtualEnv{
		PythonVersion: tokens[1],
		// For now we just detect this -- will add an option later to let user control it
		PythonFlavour: flavour,
		Name:          tokens[2],
	}
	if len(tokens) > 3 {
		env.RequirementsFile = tokens[3]
	}
	if err := validateVirtualEnv(env); err != nil {
		return nil, err
	}
	env.PythonMajorMinor = extractPythonMajorMinor(env.PythonVersion)

	env.Py37OrOlder = semver.New(env.PythonMajorMinor + ".0").LessThan(v3_8)
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

func (t *Transformer) addPythonVersion(venv *virtualEnv) (*parser.Node, error) {
	if _, exists := t.pythonVersions[venv.PythonMajorMinor]; exists {
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
	t.pythonVersions[venv.PythonMajorMinor] = struct{}{}
	return parsedNodes.AST, nil
}

func (t *Transformer) addVirtualEnvironment(venv *virtualEnv) (*parser.Node, error) {
	if _, exists := t.virtualEnvs[venv.Name]; exists {
		return nil, fmt.Errorf("")
	}
	tpl, err := template.New("venv").Parse(virtualEnvTemplate)
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	params := struct {
		*virtualEnv
		AstroUid int
	}{
		venv,
		defaultAstroUid,
	}

	if err := tpl.Execute(buf, params); err != nil {
		return nil, err
	}

	parsedNodes, err := parser.Parse(buf)
	if err != nil {
		return nil, err
	}
	return parsedNodes.AST, nil
}

func (t *Transformer) ensureValidBaseImage(node *parser.Node) error {
	var img string

	inst, err := instructions.ParseInstruction(node)
	if err != nil {
		return nil
	}
	if stage, ok := inst.(*instructions.Stage); ok {
		img = stage.BaseName
	} else {
		return nil
	}

	// TODO: To get it "back" to a node, we need to handle `--platform` here correctly :(
	imgNode := node.Next

	if strings.Contains(img, "$") {
		// Interpolate build args
		lexer := shell.NewLex('\\')
		if img, err = lexer.ProcessWordWithMap(img, t.buildArgs); err != nil {
			return err
		}

	}

	ref, err := reference.ParseNormalizedNamed(img)
	if err != nil {
		return err
	}

	if ref.Name() == astroRuntimeImage {
		if tagged, ok := ref.(reference.NamedTagged); ok {
			tag := tagged.Tag()
			if !strings.HasSuffix(tag, "-base") {
				t.runtimeImageVersion = tag
				ref, _ = reference.WithTag(ref, tag+"-base")
				imgNode.Value = ref.String()
			} else {
				t.runtimeImageVersion = strings.TrimSuffix(tag, "-base")
			}
		}
	}
	return nil
}
