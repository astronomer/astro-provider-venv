package render

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRenderDockerfileTemplate(t *testing.T) {
	tests := []struct {
		preamble string
		body     string
	}{
		{
			preamble: "ARG baseimage\nFROM ${baseimage}\n",
			body:     "RUN mkdir -p /tmp/foo\nCOPY foo bar\n",
		},
	}
	for _, tt := range tests {
		rendered, err := RenderDockerfileTemplate(tt.preamble, tt.body)
		require.NoError(t, err)
		assert.Contains(t, string(rendered), "ARG baseimage\nFROM ${baseimage}\n")
		assert.Contains(t, string(rendered), "COPY packages.txt .")
		assert.Contains(t, string(rendered), "RUN mkdir -p /tmp/foo\nCOPY foo bar\n")
	}
}
