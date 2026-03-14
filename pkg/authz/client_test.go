package authz

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	httplib "github.com/milan604/core-lab/pkg/http"
)

func TestClientDecidePostsToInternalAuthorizationEndpoint(t *testing.T) {
	client := &Client{
		baseURL: "http://iam.test/control-plane",
		httpClient: httplib.NewClient(httplib.WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/control-plane/internal/api/v1/authz/decide" {
					t.Fatalf("path = %q", req.URL.Path)
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				if !strings.Contains(string(body), `"subject_user_id":"user-1"`) {
					t.Fatalf("unexpected request body: %s", string(body))
				}
				return jsonResponse(http.StatusOK, `{"allowed":true,"subject_user_id":"user-1","service":"sites","permission_code":"sites-pages-read"}`), nil
			}),
		})),
	}

	resp, err := client.Decide(context.Background(), DecisionRequest{
		SubjectUserID: "user-1",
		Service:       "sites",
		Category:      "pages",
		Action:        "read",
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if !resp.Allowed {
		t.Fatalf("expected allowed decision")
	}
	if resp.PermissionCode != "sites-pages-read" {
		t.Fatalf("permission code = %q", resp.PermissionCode)
	}
}

func TestValidateDecisionRequestRequiresSubject(t *testing.T) {
	err := validateDecisionRequest(DecisionRequest{
		Service:  "sites",
		Category: "pages",
		Action:   "read",
	})
	if err == nil || !strings.Contains(err.Error(), "subject_user_id") {
		t.Fatalf("validateDecisionRequest() error = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
