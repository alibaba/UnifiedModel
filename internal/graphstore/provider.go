package graphstore

import (
	"fmt"
	"sync"

	"github.com/alibaba/UnifiedModel/pkg/contract"
)

const (
	ProviderTypeMemory     = "memory"
	ProviderTypeFileMemory = "file.memory"
	ProviderTypeLadybug    = "local.ladybug"
	DefaultProviderType    = ProviderTypeFileMemory
)

type ProviderConfig struct {
	Type     string
	DataRoot string
	Options  map[string]string
}

type ProviderFactory func(config ProviderConfig) (contract.GraphStore, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]ProviderFactory{}
)

func RegisterProvider(providerType string, factory ProviderFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[providerType] = factory
}

func NewProvider(config ProviderConfig) (contract.GraphStore, error) {
	providerType := config.Type
	if providerType == "" {
		providerType = DefaultProviderType
	}

	registryMu.RLock()
	factory := registry[providerType]
	registryMu.RUnlock()
	if factory == nil {
		return nil, fmt.Errorf("graphstore provider %q is not registered", providerType)
	}
	return factory(config)
}

func RegisteredProviders() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	providers := make([]string, 0, len(registry))
	for providerType := range registry {
		providers = append(providers, providerType)
	}
	return providers
}

func init() {
	RegisterProvider(ProviderTypeMemory, func(config ProviderConfig) (contract.GraphStore, error) {
		return NewMemoryStore(), nil
	})
	RegisterProvider(ProviderTypeFileMemory, func(config ProviderConfig) (contract.GraphStore, error) {
		return NewFileMemoryStore(config)
	})
}
