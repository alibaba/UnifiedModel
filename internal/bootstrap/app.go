package bootstrap

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/alibaba/UnifiedModel/internal/agentgateway"
	"github.com/alibaba/UnifiedModel/internal/entitystore"
	"github.com/alibaba/UnifiedModel/internal/graphstore"
	_ "github.com/alibaba/UnifiedModel/internal/graphstore/provider/ladybug"
	"github.com/alibaba/UnifiedModel/internal/query"
	"github.com/alibaba/UnifiedModel/internal/sampledata"
	"github.com/alibaba/UnifiedModel/internal/search"
	"github.com/alibaba/UnifiedModel/internal/umodel"
	"github.com/alibaba/UnifiedModel/internal/workspace"
	"github.com/alibaba/UnifiedModel/pkg/contract"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

type App struct {
	Workspace    *workspace.Service
	GraphStore   contract.GraphStore
	UModel       *umodel.Service
	EntityStore  *entitystore.Service
	Samples      *sampledata.Service
	Query        *query.Service
	Search       *search.Service
	AgentGateway *agentgateway.Service
}

func NewApp(dataRoot string) *App {
	app, err := NewAppWithGraphStore(dataRoot, graphstore.ProviderConfig{DataRoot: dataRoot})
	if err != nil {
		panic(err)
	}
	return app
}

func NewMemoryApp(dataRoot string) *App {
	app, err := NewAppWithGraphStore(dataRoot, graphstore.ProviderConfig{Type: graphstore.ProviderTypeMemory, DataRoot: dataRoot})
	if err != nil {
		panic(err)
	}
	return app
}

func NewFileMemoryApp(dataRoot string) *App {
	app, err := NewAppWithGraphStore(dataRoot, graphstore.ProviderConfig{Type: graphstore.ProviderTypeFileMemory, DataRoot: dataRoot})
	if err != nil {
		panic(err)
	}
	return app
}

func NewAppWithGraphStore(dataRoot string, config graphstore.ProviderConfig) (*App, error) {
	if config.DataRoot == "" {
		config.DataRoot = dataRoot
	}
	providerType := config.Type
	if providerType == "" {
		providerType = graphstore.DefaultProviderType
		config.Type = providerType
	}
	workspaceSvc := workspace.NewService(dataRoot, nil)
	if providerType == graphstore.ProviderTypeFileMemory || providerType == graphstore.ProviderTypeLadybug {
		var err error
		workspaceSvc, err = workspace.NewPersistentServiceForProvider(dataRoot, nil, providerType)
		if err != nil {
			return nil, fmt.Errorf("create workspace metadata store: %w", err)
		}
	}
	graph, err := graphstore.NewProvider(config)
	if err != nil {
		return nil, fmt.Errorf("create graphstore provider: %w", err)
	}
	searchProvider, err := search.NewProvider(search.ProviderConfig{Type: search.ProviderTypeMemory, DataRoot: dataRoot})
	if err != nil {
		return nil, fmt.Errorf("create search provider: %w", err)
	}
	searchSvc := search.NewService(searchProvider, nil, search.ProviderTypeMemory)
	umodelSvc := umodel.NewService(graph, umodel.WithSearchIndexer(searchSvc))
	entitySvc := entitystore.NewService(graph, umodelSvc, entitystore.WithSearchIndexer(searchSvc))
	sampleSvc := sampledata.NewService(umodelSvc, entitySvc)
	querySvc := query.NewServiceWithSearch(graph, searchSvc)
	agentSvc := agentgateway.NewService(querySvc, agentgateway.WithWriteServices(umodelSvc, entitySvc))

	return &App{
		Workspace:    workspaceSvc,
		GraphStore:   graph,
		UModel:       umodelSvc,
		EntityStore:  entitySvc,
		Samples:      sampleSvc,
		Query:        querySvc,
		Search:       searchSvc,
		AgentGateway: agentSvc,
	}, nil
}

func (a *App) Handler() http.Handler {
	return withCORS(a.apiMux(true))
}

func (a *App) HandlerWithUI(uiDir string) http.Handler {
	if strings.TrimSpace(uiDir) == "" {
		return withCORS(a.apiMux(true))
	}

	api := a.apiMux(false)
	ui := spaFileHandler(uiDir)
	return withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPIPath(r.URL.Path) {
			api.ServeHTTP(w, r)
			return
		}
		ui(w, r)
	}))
}

func (a *App) apiMux(includeRoot bool) *http.ServeMux {
	mux := http.NewServeMux()
	if includeRoot {
		mux.HandleFunc("/", a.handleRoot)
	}
	mux.HandleFunc("/healthz", a.handleHealth)
	mux.HandleFunc("/api/v1/capabilities", a.handleCapabilities)
	mux.HandleFunc("/api/v1/workspaces", a.handleWorkspaces)
	mux.HandleFunc("/api/v1/workspaces/", a.handleWorkspace)
	mux.HandleFunc("/api/v1/umodel/", a.handleUModel)
	mux.HandleFunc("/api/v1/entitystore/", a.handleEntityStore)
	mux.HandleFunc("/api/v1/samples/", a.handleSamples)
	mux.HandleFunc("/api/v1/query/", a.handleQuery)
	mux.HandleFunc("/api/v1/agent/", a.handleAgent)
	return mux
}

func isAPIPath(path string) bool {
	return path == "/healthz" || strings.HasPrefix(path, "/api/v1/")
}

func spaFileHandler(uiDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "method not allowed"))
			return
		}
		cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if cleanPath == "." || cleanPath == "" {
			cleanPath = "index.html"
		}
		file := filepath.Join(uiDir, filepath.FromSlash(cleanPath))
		if info, err := os.Stat(file); err == nil && !info.IsDir() {
			http.ServeFile(w, r, file)
			return
		}
		if strings.Contains(path.Base(cleanPath), ".") {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(uiDir, "index.html"))
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if applyCORSHeaders(w, r) && r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func applyCORSHeaders(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" || !allowedCORSOrigin(origin) {
		return false
	}
	headers := w.Header()
	headers.Set("Access-Control-Allow-Origin", origin)
	headers.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
	if requestHeaders := r.Header.Get("Access-Control-Request-Headers"); requestHeaders != "" {
		headers.Set("Access-Control-Allow-Headers", requestHeaders)
	} else {
		headers.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	}
	headers.Set("Access-Control-Max-Age", "600")
	headers.Add("Vary", "Origin")
	headers.Add("Vary", "Access-Control-Request-Headers")
	return true
}

func allowedCORSOrigin(origin string) bool {
	for _, allowed := range strings.Split(os.Getenv("UMODEL_CORS_ORIGINS"), ",") {
		allowed = strings.TrimSpace(allowed)
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (a *App) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "method not allowed"))
		return
	}
	health, err := a.GraphStore.Health(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"service":    "umodel-server",
		"status":     "ok",
		"graphstore": health,
		"endpoints": map[string]string{
			"health":       "/healthz",
			"workspaces":   "/api/v1/workspaces",
			"samples":      "/api/v1/samples/{workspace}/multi-domain-quickstart:import",
			"query":        "/api/v1/query/{workspace}/execute",
			"queryExplain": "/api/v1/query/{workspace}/explain",
			"agent":        "/api/v1/agent/{workspace}/discover",
		},
	})
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "method not allowed"))
		return
	}
	health, err := a.GraphStore.Health(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "graphstore": health})
}

// serviceVersion is the unified-model service version reported via
// /api/v1/capabilities. Build tooling can override at link time via
// -ldflags "-X github.com/alibaba/UnifiedModel/internal/bootstrap.serviceVersion=<v>".
var serviceVersion = "dev"

// handleCapabilities reports what query modes this server supports.
// unified-model is plan-only by design; umodel-assistant supports plan and data.
// See docs/en/spec/plan-schema-v1.md for the shared mode protocol.
func (a *App) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "method not allowed"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"service":         "unified-model",
		"version":         serviceVersion,
		"modes_supported": []string{"plan"},
		"default_mode":    "plan",
	})
}

func (a *App) handleWorkspaces(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req model.CreateWorkspaceRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		metadata, err := a.Workspace.CreateWorkspace(r.Context(), req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, metadata)
	case http.MethodGet:
		page, err := a.Workspace.ListWorkspaces(r.Context(), model.WorkspaceListRequest{
			PageRequest: model.PageRequest{
				Limit:     parseIntDefault(r.URL.Query().Get("page_size"), 100),
				PageToken: r.URL.Query().Get("page_token"),
			},
			IncludeDeleted:   r.URL.Query().Get("include_deleted") == "true",
			IncludeConflicts: r.URL.Query().Get("include_conflicts") == "true",
		})
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, page)
	default:
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "method not allowed"))
	}
}

func (a *App) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/workspaces/")
	id = strings.Trim(id, "/")
	if id == "" {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "workspace id is required"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		metadata, err := a.Workspace.GetWorkspace(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, metadata)
	case http.MethodPut:
		var req model.UpdateWorkspaceRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		metadata, err := a.Workspace.UpdateWorkspace(r.Context(), id, req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, metadata)
	case http.MethodDelete:
		metadata, err := a.Workspace.DeleteWorkspace(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, metadata)
	default:
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "method not allowed"))
	}
}

func (a *App) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "method not allowed"))
		return
	}
	workspaceID, action, ok := workspaceAction(r.URL.Path, "/api/v1/query/")
	if !ok {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "invalid query path"))
		return
	}
	var req model.QueryRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Mode == "" {
		if mode := r.URL.Query().Get("mode"); mode != "" {
			req.Mode = mode
		}
	}
	switch action {
	case "execute":
		result, err := a.Query.Execute(r.Context(), workspaceID, req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, model.NewQueryExecuteResponse(result))
	case "explain":
		explain, err := a.Query.Explain(r.Context(), workspaceID, req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, explain)
	default:
		writeError(w, apperrors.New(apperrors.CodeNotFound, "query action not found"))
	}
}

func (a *App) handleUModel(w http.ResponseWriter, r *http.Request) {
	workspaceID, action, ok := workspaceAction(r.URL.Path, "/api/v1/umodel/")
	if !ok {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "invalid umodel path"))
		return
	}
	switch {
	case r.Method == http.MethodPost && action == "import":
		var req model.UModelImportRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		result, err := a.UModel.Import(r.Context(), workspaceID, req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	case r.Method == http.MethodPost && action == "validate":
		var req struct {
			Elements []model.UModelElement `json:"elements"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		result, err := a.UModel.Validate(r.Context(), workspaceID, req.Elements)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	case r.Method == http.MethodPost && action == "elements":
		var req struct {
			Elements []model.UModelElement `json:"elements"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		result, err := a.UModel.PutElements(r.Context(), model.UModelElementBatch{Workspace: workspaceID, Elements: req.Elements})
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	case r.Method == http.MethodDelete && action == "elements":
		var req struct {
			IDs []string `json:"ids"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		result, err := a.UModel.DeleteElements(r.Context(), workspaceID, req.IDs)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeError(w, apperrors.New(apperrors.CodeNotFound, "umodel action not found"))
	}
}

func (a *App) handleEntityStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "method not allowed"))
		return
	}
	workspaceID, action, ok := workspaceAction(r.URL.Path, "/api/v1/entitystore/")
	if !ok {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "invalid entitystore path"))
		return
	}
	switch action {
	case "entities:write":
		var req model.EntityWriteBatch
		if !decodeJSON(w, r, &req) {
			return
		}
		result, err := a.EntityStore.WriteEntities(r.Context(), workspaceID, req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "entities:expire":
		var req model.ExpireRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		result, err := a.EntityStore.ExpireEntities(r.Context(), workspaceID, req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "relations:write":
		var req model.RelationWriteBatch
		if !decodeJSON(w, r, &req) {
			return
		}
		result, err := a.EntityStore.WriteRelations(r.Context(), workspaceID, req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "relations:expire":
		var req model.ExpireRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		result, err := a.EntityStore.ExpireRelations(r.Context(), workspaceID, req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeError(w, apperrors.New(apperrors.CodeNotFound, "entitystore action not found"))
	}
}

func (a *App) handleSamples(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "method not allowed"))
		return
	}
	workspaceID, action, ok := workspaceAction(r.URL.Path, "/api/v1/samples/")
	if !ok {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "invalid sample path"))
		return
	}
	sample, importAction, ok := strings.Cut(action, ":")
	if !ok || importAction != "import" {
		writeError(w, apperrors.New(apperrors.CodeNotFound, "sample action not found"))
		return
	}
	result, err := a.Samples.Import(r.Context(), workspaceID, sample)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID, action, ok := workspaceAction(r.URL.Path, "/api/v1/agent/")
	if !ok {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "invalid agent path"))
		return
	}
	switch {
	case r.Method == http.MethodGet && action == "discover":
		discovery, err := a.AgentGateway.Discover(r.Context(), workspaceID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, discovery)
	case r.Method == http.MethodPost && action == "tools:execute":
		var req model.AgentToolCallRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		result, err := a.AgentGateway.ExecuteTool(r.Context(), workspaceID, req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	case r.Method == http.MethodPost && action == "resources:read":
		var req model.AgentResourceReadRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		result, err := a.AgentGateway.ReadResource(r.Context(), workspaceID, req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeError(w, apperrors.New(apperrors.CodeNotFound, "agent action not found"))
	}
}

func workspaceAction(path, prefix string) (string, string, bool) {
	rest := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "invalid json body"))
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error) {
	if appErr, ok := apperrors.As(err); ok {
		writeJSON(w, httpStatus(appErr.Code), map[string]any{"error": appErr})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{
		"error": apperrors.New(apperrors.CodeInternal, err.Error()),
	})
}

func httpStatus(code apperrors.Code) int {
	switch code {
	case apperrors.CodeInvalidArgument, apperrors.CodeValidationFailed, apperrors.CodeQueryParseError, apperrors.CodeQueryPlanError:
		return http.StatusBadRequest
	case apperrors.CodeNotFound:
		return http.StatusNotFound
	case apperrors.CodeConflict, apperrors.CodeVersionConflict, apperrors.CodeWorkspaceTombstoned, apperrors.CodeWorkspaceConflicted:
		return http.StatusConflict
	case apperrors.CodeProviderUnavailable:
		return http.StatusServiceUnavailable
	case apperrors.CodeToolDisabled, apperrors.CodeToolNotFound:
		return http.StatusBadRequest
	case apperrors.CodeProviderUnsupported, apperrors.CodeNotImplemented:
		return http.StatusNotImplemented
	case apperrors.CodeTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

func parseIntDefault(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	var n int
	for _, r := range value {
		if r < '0' || r > '9' {
			return fallback
		}
		n = n*10 + int(r-'0')
	}
	if n <= 0 {
		return fallback
	}
	return n
}
