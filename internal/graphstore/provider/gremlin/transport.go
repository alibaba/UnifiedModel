package gremlin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const graphSONMime = "application/vnd.gremlin-v3.0+json"

type result struct{ Data any }

// submit holds the provider gate for the entire synchronous request, including
// all partial responses and SASL challenges. No background connection pool can
// race a reader against shutdown/reconnect, and writes are never replayed.
func (p *Provider) submit(ctx context.Context, script string, bindings map[string]any) ([]*result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	deadline, _ := ctx.Deadline()
	ms := max(int((time.Until(deadline)+time.Millisecond-1)/time.Millisecond), 1)
	id := uuid.NewString()
	packet, err := requestPacket(id, "eval", map[string]any{
		"gremlin": script, "language": "gremlin-groovy", "bindings": bindings,
		"aliases": map[string]any{"g": p.settings.traversalSource}, "evaluationTimeout": ms,
	})
	if err != nil {
		return nil, err
	}
	if err := p.client.SetWriteDeadline(deadline); err != nil {
		p.closeLocked()
		return nil, err
	}
	if err := p.client.SetReadDeadline(deadline); err != nil {
		p.closeLocked()
		return nil, err
	}
	finished := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = p.client.Close()
		close(finished)
	})
	defer func() {
		if !stop() {
			<-finished
		}
	}()
	fail := func(err error) ([]*result, error) {
		p.closeLocked()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("gremlin: connection lost: %w", err)
	}
	if err := p.client.WriteMessage(websocket.BinaryMessage, packet); err != nil {
		return fail(err)
	}
	rows := []*result{}
	authenticated := false
	for {
		_, payload, err := p.client.ReadMessage()
		if err != nil {
			return fail(err)
		}
		var response struct {
			RequestID any `json:"requestId"`
			Status    struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"status"`
			Result struct {
				Data any `json:"data"`
			} `json:"result"`
		}
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.UseNumber()
		if err := decoder.Decode(&response); err != nil {
			return fail(fmt.Errorf("invalid GraphSON response"))
		}
		if graphSONValue(response.RequestID) != id {
			return fail(fmt.Errorf("response request ID does not match"))
		}
		if response.Status.Code == 407 {
			if authenticated || p.settings.username == "" {
				return nil, fmt.Errorf("gremlin: authentication required or failed")
			}
			authenticated = true
			sasl := base64.StdEncoding.EncodeToString([]byte("\x00" + p.settings.username + "\x00" + p.settings.password))
			packet, err := requestPacket(id, "authentication", map[string]any{"sasl": sasl, "saslMechanism": "PLAIN"})
			if err != nil {
				return nil, err
			}
			if err := p.client.WriteMessage(websocket.BinaryMessage, packet); err != nil {
				return fail(err)
			}
			continue
		}
		if response.Status.Code == 598 {
			p.closeLocked()
			return nil, context.DeadlineExceeded
		}
		if response.Status.Code != 200 && response.Status.Code != 204 && response.Status.Code != 206 {
			return nil, fmt.Errorf("gremlin: request failed (status %d): %s", response.Status.Code, response.Status.Message)
		}
		if data := graphSONValue(response.Result.Data); data != nil {
			values, ok := data.([]any)
			if !ok {
				return fail(fmt.Errorf("result data must be a GraphSON list"))
			}
			for _, v := range values {
				rows = append(rows, &result{Data: v})
			}
		}
		if response.Status.Code != 206 {
			return rows, nil
		}
	}
}

func requestPacket(id, operation string, args map[string]any) ([]byte, error) {
	body, err := json.Marshal(map[string]any{"requestId": id, "op": operation, "processor": "", "args": graphSONEncode(args)})
	if err != nil {
		return nil, fmt.Errorf("gremlin: encode request: %w", err)
	}
	if len(body)+1+len(graphSONMime) > 1<<20 {
		return nil, fmt.Errorf("gremlin: request exceeds 1 MiB; split the write batch")
	}
	packet := append([]byte{byte(len(graphSONMime))}, graphSONMime...)
	return append(packet, body...), nil
}

// Request envelopes and nested bindings follow GraphSON 3's explicit Map/List
// types. The provider's persisted payloads are JSON strings, not graph elements.
func graphSONEncode(v any) any {
	typed := func(kind string, value any) any { return map[string]any{"@type": "g:" + kind, "@value": value} }
	switch v := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		values := make([]any, 0, len(v)*2)
		for _, k := range keys {
			values = append(values, k, graphSONEncode(v[k]))
		}
		return typed("Map", values)
	case []string:
		values := make([]any, len(v))
		for i, s := range v {
			values[i] = s
		}
		return typed("List", values)
	case []map[string]any:
		values := make([]any, len(v))
		for i, m := range v {
			values[i] = graphSONEncode(m)
		}
		return typed("List", values)
	case int:
		return typed("Int64", v)
	default:
		return v
	}
}

func graphSONValue(v any) any {
	if list, ok := v.([]any); ok {
		for i, item := range list {
			list[i] = graphSONValue(item)
		}
		return list
	}
	typed, ok := v.(map[string]any)
	if !ok || typed["@type"] == nil {
		return v
	}
	value := typed["@value"]
	switch typed["@type"] {
	case "g:Int32", "g:Int64":
		if n, ok := value.(json.Number); ok {
			i, err := strconv.ParseInt(string(n), 10, 64)
			if err == nil {
				return i
			}
		}
	}
	return graphSONValue(value)
}
