package search

import (
	"fmt"
	"sync"

	searchcontract "github.com/alibaba/UnifiedModel/internal/search/contract"
)

const (
	ProviderTypeMemory  = "memory"
	DefaultProviderType = ProviderTypeMemory
)

type ProviderConfig struct {
	Type     string
	DataRoot string
	Options  map[string]string
}

type ProviderFactory func(config ProviderConfig) (searchcontract.Provider, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]ProviderFactory{}
)

func RegisterProvider(providerType string, factory ProviderFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[providerType] = factory
}

func NewProvider(config ProviderConfig) (searchcontract.Provider, error) {
	providerType := config.Type
	if providerType == "" {
		providerType = DefaultProviderType
	}

	registryMu.RLock()
	factory := registry[providerType]
	registryMu.RUnlock()
	if factory == nil {
		return nil, fmt.Errorf("search provider %q is not registered", providerType)
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
	RegisterProvider(ProviderTypeMemory, func(config ProviderConfig) (searchcontract.Provider, error) {
		return NewMemoryProvider(), nil
	})
}
