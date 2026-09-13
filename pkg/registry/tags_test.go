package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTagsFollowsLinkPagination(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.RawQuery {
		case "":
			w.Header().Set("Link", `</v2/lib/app/tags/list?n=2&last=b>; rel="next"`)
			w.Write([]byte(`{"name":"lib/app","tags":["a","b"]}`))
		case "n=2&last=b":
			w.Header().Set("Link", `</v2/lib/app/tags/list?n=2&last=d>; rel="next"`)
			w.Write([]byte(`{"name":"lib/app","tags":["c","d"]}`))
		default:
			w.Write([]byte(`{"name":"lib/app","tags":["e"]}`))
		}
	}))
	defer srv.Close()
	old := client
	client = srv.Client()
	defer func() { client = old }()

	host := strings.TrimPrefix(srv.URL, "https://")
	tags, err := Tags(context.Background(), host+"/lib/app")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tags, ","); got != "a,b,c,d,e" {
		t.Fatalf("tags = %s", got)
	}
}

func TestNextLink(t *testing.T) {
	cur := "https://registry-1.docker.io/v2/library/postgres/tags/list"
	if got := nextLink(cur, `</v2/library/postgres/tags/list?last=16&n=100>; rel="next"`); got != "https://registry-1.docker.io/v2/library/postgres/tags/list?last=16&n=100" {
		t.Fatalf("relative: %q", got)
	}
	if got := nextLink(cur, `<https://x/y>; rel="last"`); got != "" {
		t.Fatalf("non-next should be ignored: %q", got)
	}
	if got := nextLink(cur, ""); got != "" {
		t.Fatalf("empty: %q", got)
	}
}
