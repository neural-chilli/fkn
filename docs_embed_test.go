package fkn

import (
	"strings"
	"testing"
)

func TestDocReturnsEmbeddedContent(t *testing.T) {
	t.Parallel()

	got, err := Doc("user-guide")
	if err != nil {
		t.Fatalf("Doc(user-guide) error = %v", err)
	}
	if !strings.Contains(got, "# fkn User Guide") {
		t.Fatalf("Doc(user-guide) = %q, want embedded user guide", got)
	}
}

func TestDocNamesIncludesPrimaryPages(t *testing.T) {
	t.Parallel()

	names := DocNames()
	joined := strings.Join(names, ",")
	for _, want := range []string{"readme", "releasing", "user-guide"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("DocNames() = %v, want %q", names, want)
		}
	}
}
