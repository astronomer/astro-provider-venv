package dockerfile

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/command"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/pkg/errors"
)

const (
	localNameDockerfile = "dockerfile"
)

func Parse(contents []byte) (*parser.Node, error) {
	node, err := parser.Parse(bytes.NewBuffer(contents))
	if err != nil {
		return nil, err
	}
	return node.AST, nil
}

func Read(ctx context.Context, c client.Client) ([]byte, error) {
	includePatterns := []string{"Dockerfile"}
	if c.BuildOpts().Opts["filename"] != "" {
		includePatterns = []string{c.BuildOpts().Opts["filename"]}
	}
	src := llb.Local(
		localNameDockerfile,
		llb.IncludePatterns(includePatterns),
		llb.SessionID(c.BuildOpts().SessionID),
		llb.SharedKeyHint(includePatterns[0]),
		llb.WithCustomName("[astro] transform PYENV directives"),
	)
	def, err := src.Marshal(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal local source")
	}
	res, err := c.Solve(ctx, client.SolveRequest{
		Definition: def.ToPB(),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create solve request")
	}
	ref, err := res.SingleRef()
	if err != nil {
		return nil, err
	}
	return ref.ReadFile(ctx, client.ReadRequest{
		Filename: includePatterns[0],
	})
}

// Print a dockerfile from an AST node
// From https://raw.githubusercontent.com/tilt-dev/tilt/e9ab890264fab4c81db3d228e57e6f3d1a85f707/internal/dockerfile/ast.go
func Print(node *parser.Node) (string, error) {
	buf := bytes.NewBuffer(nil)
	currentLine := 1
	for _, node := range node.Children {
		for currentLine < node.StartLine {
			_, err := buf.Write([]byte("\n"))
			if err != nil {
				return "", err
			}
			currentLine++
		}

		err := printNode(node, buf)
		if err != nil {
			return "", err
		}

		currentLine = node.StartLine + 1
		if node.Next != nil && node.Next.StartLine != 0 {
			currentLine = node.Next.StartLine + 1
		}
	}
	return buf.String(), nil
}

func printNode(node *parser.Node, writer io.Writer) error {
	v := fmtNode(node)

	_, err := fmt.Fprintln(writer, v)
	if err != nil {
		return err
	}
	return nil
}

func fmtNode(node *parser.Node) (v string) {
	// format per directive
	switch strings.ToLower(node.Value) {
	// all the commands that use parseMaybeJSON
	// https://github.com/moby/buildkit/blob/2ec7d53b00f24624cda0adfbdceed982623a93b3/frontend/dockerfile/parser/parser.go#L152
	case command.Cmd, command.Entrypoint, command.Run, command.Shell:
		v = fmtCmd(node)
	case command.Label:
		v = fmtLabel(node)
	case command.Onbuild:
		v = fmtSubCommand(node)
	default:
		v = fmtDefault(node)
	}
	return v
}

func getCmd(n *parser.Node) []string {
	if n == nil {
		return nil
	}

	cmd := []string{strings.ToUpper(n.Value)}
	if len(n.Flags) > 0 {
		cmd = append(cmd, n.Flags...)
	}

	return append(cmd, getCmdArgs(n)...)
}

func getCmdArgs(n *parser.Node) []string {
	if n == nil {
		return nil
	}

	cmd := []string{}
	for node := n.Next; node != nil; node = node.Next {
		cmd = append(cmd, node.Value)
		if len(node.Flags) > 0 {
			cmd = append(cmd, node.Flags...)
		}
	}

	return cmd
}

func fmtCmd(node *parser.Node) string {
	if node.Attributes["json"] {
		cmd := []string{strings.ToUpper(node.Value)}
		if len(node.Flags) > 0 {
			cmd = append(cmd, node.Flags...)
		}

		encoded := []string{}
		for _, c := range getCmdArgs(node) {
			encoded = append(encoded, fmt.Sprintf("%q", c))
		}
		return fmt.Sprintf("%s [%s]", strings.Join(cmd, " "), strings.Join(encoded, ", "))
	}

	cmd := getCmd(node)
	return strings.Join(cmd, " ")
}

func fmtSubCommand(node *parser.Node) string {
	cmd := []string{strings.ToUpper(node.Value)}
	if len(node.Flags) > 0 {
		cmd = append(cmd, node.Flags...)
	}

	sub := node.Next.Children[0]
	return strings.Join(cmd, " ") + " " + fmtCmd(sub)
}

func fmtDefault(node *parser.Node) string {
	cmd := getCmd(node)
	return strings.Join(cmd, " ")
}

func fmtLabel(node *parser.Node) string {
	cmd := getCmd(node)
	assignments := []string{cmd[0]}
	for i := 1; i < len(cmd); i += 2 {
		if i+1 < len(cmd) {
			assignments = append(assignments, fmt.Sprintf("%s=%s", cmd[i], cmd[i+1]))
		} else {
			assignments = append(assignments, fmt.Sprintf("%s", cmd[i]))
		}
	}
	return strings.Join(assignments, " ")
}
