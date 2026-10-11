package main

import (
	"fmt"
	"testing"

	"github.com/kipitix/gracedown"
	"github.com/kipitix/growscada/internal/apiclient"
	"github.com/kipitix/growscada/internal/devicelink"
	"github.com/kipitix/growscada/internal/devicesim"
)

func TestDeviceExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"missing tag", fmt.Errorf("resolve: %w", devicelink.ErrTagNotFound), gracedown.ExitConfig},
		{"incompatible pattern", fmt.Errorf("connect: %w", devicesim.ErrPatternIncompatible), gracedown.ExitConfig},
		{"unauthorized", &apiclient.StatusError{Status: 401}, gracedown.ExitNoPerm},
		{"forbidden", &apiclient.StatusError{Status: 403}, gracedown.ExitNoPerm},
		{"bad request", &apiclient.StatusError{Status: 400}, gracedown.ExitProtocol},
		{"not found response", &apiclient.StatusError{Status: 404}, gracedown.ExitProtocol},
		{"server error", &apiclient.StatusError{Status: 500}, gracedown.ExitProtocol},
		{"invalid protocol", fmt.Errorf("invalid URL scheme"), gracedown.ExitProtocol},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deviceExitCode(tt.err); got != tt.want {
				t.Fatalf("deviceExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}
