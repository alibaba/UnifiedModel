package registry

import (
	"sync"
)

type ParamMeta struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Flag        string `json:"flag"`
	Short       string `json:"short,omitempty"`
	Description string `json:"description"`
	Required    bool   `json:"required,omitempty"`
	Default     string `json:"default,omitempty"`
}

type CommandMeta struct {
	Group       string      `json:"group"`
	Resource    string      `json:"resource"`
	Action      string      `json:"action"`
	Description string      `json:"description"`
	Params      []ParamMeta `json:"params,omitempty"`
}

type Registry struct {
	mu       sync.Mutex
	commands []CommandMeta
}

var Global = &Registry{}

func (r *Registry) Register(meta CommandMeta) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commands = append(r.commands, meta)
}

func (r *Registry) All() []CommandMeta {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]CommandMeta, len(r.commands))
	copy(out, r.commands)
	return out
}
