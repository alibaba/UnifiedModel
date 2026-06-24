//go:build !ladybug

package ladybug

import (
	"context"
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestStubProviderReportsUnavailable(t *testing.T) {
	provider, err := NewProvider(graphstore.ProviderConfig{})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	ctx := context.Background()
	if health, err := provider.Health(ctx); err != nil || health.Status != "unavailable" || health.Provider != graphstore.ProviderTypeLadybug {
		t.Fatalf("health: %+v err=%v", health, err)
	} else {
		requireLadybugGuidance(t, health.Message)
	}
	if err := provider.OpenWorkspace(ctx, model.WorkspaceMetadata{ID: "demo"}); !apperrors.IsCode(err, apperrors.CodeProviderUnavailable) {
		t.Fatalf("expected provider unavailable, got %v", err)
	} else {
		requireLadybugGuidance(t, err.Error())
	}
	if capabilities, err := provider.Capabilities(ctx); err != nil || !capabilities.ControlledCypher || capabilities.MaxLimit == 0 {
		t.Fatalf("stub should expose intended local.ladybug capabilities for planning, got %+v err=%v", capabilities, err)
	}
}

func TestStubProviderIsRegisteredButUnavailable(t *testing.T) {
	provider, err := graphstore.NewProvider(graphstore.ProviderConfig{Type: graphstore.ProviderTypeLadybug})
	if err != nil {
		t.Fatalf("new registered provider: %v", err)
	}
	if err := provider.EnsureSchema(context.Background(), "demo"); !apperrors.IsCode(err, apperrors.CodeProviderUnavailable) {
		t.Fatalf("expected registered stub provider to be unavailable, got %v", err)
	} else {
		requireLadybugGuidance(t, err.Error())
	}
}

func requireLadybugGuidance(t *testing.T, message string) {
	t.Helper()
	for _, want := range []string{"-tags ladybug", "--graphstore file.memory"} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q guidance in %q", want, message)
		}
	}
}
