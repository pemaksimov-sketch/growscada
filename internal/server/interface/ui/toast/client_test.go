package toast

import (
	"fmt"
	"testing"

	"github.com/kipitix/growscada/internal/apiclient"
)

func TestFromClientError_PreservesWrappedProblem(t *testing.T) {
	err := fmt.Errorf("save: %w", &apiclient.StatusError{Status: 409, Title: "Conflict", Detail: "stale scene", Instance: "/api/v0/scenes/one", Method: "PUT"})
	got := FromClientError(err)
	want := Problem{Status: 409, Title: "Conflict", Detail: "stale scene", Instance: "/api/v0/scenes/one", Method: "PUT"}
	if got != want {
		t.Fatalf("problem lost fields: got %+v want %+v", got, want)
	}
}

func TestFromClientError_StatusWithoutProblemAndTransportFailure(t *testing.T) {
	got := FromClientError(&apiclient.StatusError{Status: 503})
	if got.Title != "Service Unavailable" || got.Status != 503 {
		t.Fatalf("missing HTTP fallback: %+v", got)
	}
	got = FromClientError(fmt.Errorf("connection closed"))
	if got.Status != 0 || got.Detail != "connection closed" {
		t.Fatalf("missing transport failure: %+v", got)
	}
}
