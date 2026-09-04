package renderer

import (
	"io"
	"strings"
	"testing"
)

func TestTemplatePartialsCanBeClonedForAuxiliaryDocuments(t *testing.T) {
	buffered, err := bufferTemplatePartials(map[string]io.Reader{
		"brand": strings.NewReader("<strong>Abera</strong>"),
	})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		partials := cloneTemplatePartials(buffered)
		content, err := io.ReadAll(partials["brand"])
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "<strong>Abera</strong>" {
			t.Fatalf("unexpected partial content: %q", content)
		}
	}
}
