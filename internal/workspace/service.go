package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

var workspaceIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_-]{0,62}[a-z0-9])?$`)

const (
	providerTypeFileMemory = "file.memory"
	providerTypeLadybug    = "local.ladybug"
)

type Service struct {
	mu          sync.RWMutex
	root        string
	statePath   string
	workspaces  map[string]model.WorkspaceMetadata
	configRules WorkspaceConfigValidator
}

type persistedWorkspaceState struct {
	Version int                                `json:"version"`
	Items   map[string]model.WorkspaceMetadata `json:"items"`
}

type WorkspaceConfigValidator interface {
	ValidateNamespace(ctx context.Context, namespace string, value map[string]any) error
}

func NewService(root string, configRules WorkspaceConfigValidator) *Service {
	return &Service{
		root:        strings.TrimRight(root, "/"),
		workspaces:  make(map[string]model.WorkspaceMetadata),
		configRules: configRules,
	}
}

func NewPersistentService(root string, configRules WorkspaceConfigValidator) (*Service, error) {
	return NewPersistentServiceForProvider(root, configRules, providerTypeFileMemory)
}

// NewPersistentServiceForProvider creates a persistent workspace metadata
// service and recovers missing metadata from provider-specific storage paths.
func NewPersistentServiceForProvider(root string, configRules WorkspaceConfigValidator, providerType string) (*Service, error) {
	s := NewService(root, configRules)
	s.statePath = filepath.Join(s.rootOrDefault(), "workspaces.json")
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	recovered, err := s.recoverProviderWorkspaceDirsLocked(providerType)
	if err != nil {
		return nil, err
	}
	if recovered {
		if err := s.persistLocked(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Service) CreateWorkspace(ctx context.Context, req model.CreateWorkspaceRequest) (model.WorkspaceMetadata, error) {
	if err := validateWorkspaceID(req.ID); err != nil {
		return model.WorkspaceMetadata{}, err
	}
	if err := validateLabels(req.Labels); err != nil {
		return model.WorkspaceMetadata{}, err
	}
	if err := s.validateConfig(ctx, req.Config); err != nil {
		return model.WorkspaceMetadata{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.workspaces[req.ID]; ok {
		if existing.Status == model.WorkspaceStatusDeleted {
			return model.WorkspaceMetadata{}, apperrors.New(apperrors.CodeWorkspaceTombstoned, "workspace metadata is tombstoned")
		}
		return model.WorkspaceMetadata{}, apperrors.New(apperrors.CodeConflict, "workspace already exists")
	}

	now := time.Now().UTC()
	name := req.Name
	if name == "" {
		name = req.ID
	}
	metadata := model.WorkspaceMetadata{
		ID:              req.ID,
		Name:            name,
		Description:     req.Description,
		Labels:          cloneStringMap(req.Labels),
		Config:          cloneConfig(req.Config),
		Paths:           model.WorkspacePaths{Root: s.workspaceRoot(req.ID), Tmp: s.workspaceRoot(req.ID) + "/tmp"},
		Status:          model.WorkspaceStatusActive,
		ResourceVersion: 1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	s.workspaces[req.ID] = metadata
	if err := s.persistLocked(); err != nil {
		delete(s.workspaces, req.ID)
		return model.WorkspaceMetadata{}, err
	}
	return metadata, nil
}

func (s *Service) GetWorkspace(ctx context.Context, id string) (model.WorkspaceMetadata, error) {
	if err := validateWorkspaceID(id); err != nil {
		return model.WorkspaceMetadata{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	metadata, ok := s.workspaces[id]
	if !ok {
		return model.WorkspaceMetadata{}, apperrors.New(apperrors.CodeNotFound, "workspace not found")
	}
	switch metadata.Status {
	case model.WorkspaceStatusDeleted:
		return model.WorkspaceMetadata{}, apperrors.New(apperrors.CodeWorkspaceTombstoned, "workspace metadata is tombstoned")
	case model.WorkspaceStatusConflicted:
		return model.WorkspaceMetadata{}, apperrors.New(apperrors.CodeWorkspaceConflicted, "workspace identity is conflicted")
	default:
		return metadata, nil
	}
}

func (s *Service) ListWorkspaces(ctx context.Context, req model.WorkspaceListRequest) (model.Page[model.WorkspaceMetadata], error) {
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.workspaces))
	for id := range s.workspaces {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	start := 0
	if req.PageToken != "" {
		for i, id := range ids {
			if id == req.PageToken {
				start = i + 1
				break
			}
		}
	}

	items := make([]model.WorkspaceMetadata, 0, limit)
	next := ""
	for _, id := range ids[start:] {
		metadata := s.workspaces[id]
		if metadata.Status == model.WorkspaceStatusDeleted && !req.IncludeDeleted {
			continue
		}
		if metadata.Status == model.WorkspaceStatusConflicted && !req.IncludeConflicts {
			continue
		}
		if !matchesLabels(metadata.Labels, req.LabelSelector) {
			continue
		}
		if len(items) == limit {
			next = id
			break
		}
		items = append(items, metadata)
	}

	return model.Page[model.WorkspaceMetadata]{Items: items, NextToken: next}, nil
}

func (s *Service) UpdateWorkspace(ctx context.Context, id string, req model.UpdateWorkspaceRequest) (model.WorkspaceMetadata, error) {
	if err := validateWorkspaceID(id); err != nil {
		return model.WorkspaceMetadata{}, err
	}
	if err := validateLabels(req.Labels); err != nil {
		return model.WorkspaceMetadata{}, err
	}
	if err := s.validateConfig(ctx, req.Config); err != nil {
		return model.WorkspaceMetadata{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	metadata, ok := s.workspaces[id]
	if !ok {
		return model.WorkspaceMetadata{}, apperrors.New(apperrors.CodeNotFound, "workspace not found")
	}
	if metadata.Status == model.WorkspaceStatusDeleted {
		return model.WorkspaceMetadata{}, apperrors.New(apperrors.CodeWorkspaceTombstoned, "workspace metadata is tombstoned")
	}
	if metadata.Status == model.WorkspaceStatusConflicted {
		return model.WorkspaceMetadata{}, apperrors.New(apperrors.CodeWorkspaceConflicted, "workspace identity is conflicted")
	}
	if req.IfMatchVersion != 0 && req.IfMatchVersion != metadata.ResourceVersion {
		return model.WorkspaceMetadata{}, apperrors.New(apperrors.CodeVersionConflict, "workspace resource version mismatch")
	}

	changed := false
	if req.Name != nil && metadata.Name != *req.Name {
		metadata.Name = *req.Name
		changed = true
	}
	if req.Description != nil && metadata.Description != *req.Description {
		metadata.Description = *req.Description
		changed = true
	}
	if req.ReplaceLabels {
		if !stringMapsEqual(metadata.Labels, req.Labels) {
			metadata.Labels = cloneStringMap(req.Labels)
			changed = true
		}
	} else {
		labels, labelsChanged := mergeStringMap(metadata.Labels, req.Labels)
		if labelsChanged {
			metadata.Labels = labels
			changed = true
		}
	}
	if req.ReplaceConfig {
		metadata.Config = cloneConfig(req.Config)
		changed = true
	} else {
		config, configChanged := mergeConfig(metadata.Config, req.Config)
		if configChanged {
			metadata.Config = config
			changed = true
		}
	}

	if changed {
		previous := s.workspaces[id]
		metadata.ResourceVersion++
		metadata.UpdatedAt = time.Now().UTC()
		s.workspaces[id] = metadata
		if err := s.persistLocked(); err != nil {
			s.workspaces[id] = previous
			return model.WorkspaceMetadata{}, err
		}
	}
	return metadata, nil
}

func (s *Service) DeleteWorkspace(ctx context.Context, id string) (model.WorkspaceMetadata, error) {
	if err := validateWorkspaceID(id); err != nil {
		return model.WorkspaceMetadata{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	metadata, ok := s.workspaces[id]
	if !ok {
		return model.WorkspaceMetadata{}, apperrors.New(apperrors.CodeNotFound, "workspace not found")
	}
	if metadata.Status == model.WorkspaceStatusConflicted {
		return model.WorkspaceMetadata{}, apperrors.New(apperrors.CodeWorkspaceConflicted, "workspace identity is conflicted")
	}
	if metadata.Status == model.WorkspaceStatusDeleted {
		return metadata, nil
	}
	now := time.Now().UTC()
	metadata.Status = model.WorkspaceStatusDeleted
	metadata.DeletedAt = &now
	metadata.UpdatedAt = now
	metadata.ResourceVersion++
	previous := s.workspaces[id]
	s.workspaces[id] = metadata
	if err := s.persistLocked(); err != nil {
		s.workspaces[id] = previous
		return model.WorkspaceMetadata{}, err
	}
	return metadata, nil
}

func (s *Service) rootOrDefault() string {
	if s.root == "" {
		return "data"
	}
	return s.root
}

func (s *Service) workspaceRoot(id string) string {
	if s.root == "" {
		return "data/instances/" + id
	}
	return s.root + "/instances/" + id
}

func (s *Service) loadLocked() error {
	if s.statePath == "" {
		return nil
	}
	body, err := os.ReadFile(s.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read workspace metadata state: %w", err)
	}
	var state persistedWorkspaceState
	if err := json.Unmarshal(body, &state); err != nil {
		return fmt.Errorf("decode workspace metadata state: %w", err)
	}
	if state.Version != 1 {
		return fmt.Errorf("unsupported workspace metadata state version %d", state.Version)
	}
	for id, metadata := range state.Items {
		if metadata.ID == "" {
			metadata.ID = id
		}
		if metadata.ID != id {
			return fmt.Errorf("workspace metadata key %q does not match id %q", id, metadata.ID)
		}
		if err := validateWorkspaceID(metadata.ID); err != nil {
			return err
		}
		s.normalizeMetadata(&metadata)
		s.workspaces[id] = metadata
	}
	return nil
}

func (s *Service) recoverProviderWorkspaceDirsLocked(providerType string) (bool, error) {
	switch providerType {
	case "", providerTypeFileMemory:
		return s.recoverFileMemoryWorkspaceDirsLocked()
	case providerTypeLadybug:
		return s.recoverLadybugWorkspaceDirsLocked()
	default:
		return false, nil
	}
}

func (s *Service) recoverFileMemoryWorkspaceDirsLocked() (bool, error) {
	return s.recoverWorkspaceDirsLocked(
		filepath.Join(s.rootOrDefault(), "graphstore", "file-memory", "workspaces"),
		"file memory workspace directories",
	)
}

func (s *Service) recoverLadybugWorkspaceDirsLocked() (bool, error) {
	root := filepath.Join(s.rootOrDefault(), "instances")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read ladybug workspace root: %w", err)
	}

	recovered := false
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		graphPath := filepath.Join(root, id, "storage", "graph", "local", "ladybug")
		info, err := os.Stat(graphPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, fmt.Errorf("stat ladybug workspace directory: %w", err)
		}
		if !info.IsDir() {
			continue
		}
		didRecover, err := s.recoverWorkspaceDirLocked(id, info)
		if err != nil {
			return false, err
		}
		recovered = recovered || didRecover
	}
	return recovered, nil
}

func (s *Service) recoverWorkspaceDirsLocked(root string, description string) (bool, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read %s: %w", description, err)
	}

	recovered := false
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return false, fmt.Errorf("read %s info: %w", description, err)
		}
		didRecover, err := s.recoverWorkspaceDirLocked(entry.Name(), info)
		if err != nil {
			return false, err
		}
		recovered = recovered || didRecover
	}
	return recovered, nil
}

func (s *Service) recoverWorkspaceDirLocked(id string, info os.FileInfo) (bool, error) {
	if _, exists := s.workspaces[id]; exists {
		return false, nil
	}
	if err := validateWorkspaceID(id); err != nil {
		return false, err
	}
	now := info.ModTime().UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	s.workspaces[id] = model.WorkspaceMetadata{
		ID:              id,
		Name:            id,
		Paths:           model.WorkspacePaths{Root: s.workspaceRoot(id), Tmp: s.workspaceRoot(id) + "/tmp"},
		Status:          model.WorkspaceStatusActive,
		ResourceVersion: 1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return true, nil
}

func (s *Service) normalizeMetadata(metadata *model.WorkspaceMetadata) {
	if metadata.Name == "" {
		metadata.Name = metadata.ID
	}
	if metadata.Status == "" {
		metadata.Status = model.WorkspaceStatusActive
	}
	if metadata.ResourceVersion == 0 {
		metadata.ResourceVersion = 1
	}
	if metadata.Paths.Root == "" {
		metadata.Paths.Root = s.workspaceRoot(metadata.ID)
	}
	if metadata.Paths.Tmp == "" {
		metadata.Paths.Tmp = metadata.Paths.Root + "/tmp"
	}
	now := time.Now().UTC()
	if metadata.CreatedAt.IsZero() {
		metadata.CreatedAt = now
	}
	if metadata.UpdatedAt.IsZero() {
		metadata.UpdatedAt = metadata.CreatedAt
	}
}

func (s *Service) persistLocked() error {
	if s.statePath == "" {
		return nil
	}
	state := persistedWorkspaceState{
		Version: 1,
		Items:   make(map[string]model.WorkspaceMetadata, len(s.workspaces)),
	}
	for id, metadata := range s.workspaces {
		state.Items[id] = metadata
	}
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode workspace metadata state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.statePath), 0o755); err != nil {
		return fmt.Errorf("create workspace metadata directory: %w", err)
	}
	tmp := s.statePath + ".tmp"
	if err := os.WriteFile(tmp, append(body, '\n'), 0o644); err != nil {
		return fmt.Errorf("write workspace metadata state: %w", err)
	}
	if err := os.Rename(tmp, s.statePath); err != nil {
		return fmt.Errorf("replace workspace metadata state: %w", err)
	}
	return nil
}

func (s *Service) validateConfig(ctx context.Context, config map[string]map[string]any) error {
	if s.configRules == nil {
		return nil
	}
	for namespace, value := range config {
		if err := s.configRules.ValidateNamespace(ctx, namespace, value); err != nil {
			return err
		}
	}
	return nil
}

func validateWorkspaceID(id string) error {
	if !workspaceIDPattern.MatchString(id) {
		return apperrors.WithDetails(apperrors.CodeInvalidArgument, "invalid workspace id", map[string]string{
			"rule": "^[a-z0-9](?:[a-z0-9_-]{0,62}[a-z0-9])?$",
		})
	}
	return nil
}

func validateLabels(labels map[string]string) error {
	if len(labels) > 64 {
		return apperrors.New(apperrors.CodeInvalidArgument, "labels exceed max count")
	}
	for k, v := range labels {
		if len(k) == 0 || len(k) > 63 || len(v) > 255 {
			return apperrors.New(apperrors.CodeInvalidArgument, "invalid label key or value")
		}
	}
	return nil
}

func matchesLabels(labels, selector map[string]string) bool {
	for k, expected := range selector {
		if labels[k] != expected {
			return false
		}
	}
	return true
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneConfig(in map[string]map[string]any) map[string]map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]map[string]any, len(in))
	for ns, values := range in {
		out[ns] = make(map[string]any, len(values))
		for k, v := range values {
			out[ns][k] = v
		}
	}
	return out
}

func mergeStringMap(target map[string]string, patch map[string]string) (map[string]string, bool) {
	if len(patch) == 0 {
		return target, false
	}
	if target == nil {
		target = make(map[string]string)
	}
	changed := false
	for k, v := range patch {
		if target[k] != v {
			target[k] = v
			changed = true
		}
	}
	return target, changed
}

func mergeConfig(target map[string]map[string]any, patch map[string]map[string]any) (map[string]map[string]any, bool) {
	if len(patch) == 0 {
		return target, false
	}
	if target == nil {
		target = make(map[string]map[string]any)
	}
	changed := false
	for ns, values := range patch {
		if target[ns] == nil {
			target[ns] = make(map[string]any)
		}
		for k, v := range values {
			if !reflect.DeepEqual(target[ns][k], v) {
				target[ns][k] = v
				changed = true
			}
		}
	}
	return target, changed
}

func stringMapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		if b[k] != av {
			return false
		}
	}
	return true
}
