package neo4j

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/model"
	driver "github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// This wire-level fixture deliberately speaks only Bolt v3 and rejects Neo4j
// management statements and bookmark metadata. It exercises the real driver,
// including its database-selection check, without claiming to emulate GDB.
func bolt3Fixture(t *testing.T, rows func(string) (fields []string, records [][]string)) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var connections sync.Map
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connections.Store(conn, true)
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer connections.Delete(conn)
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
				if err := serveBolt3(conn, rows); err != nil && !errors.Is(err, net.ErrClosed) {
					t.Errorf("Bolt v3 fixture: %v", err)
				}
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		connections.Range(func(conn, _ any) bool { _ = conn.(net.Conn).Close(); return true })
		wg.Wait()
	})
	return "bolt://" + listener.Addr().String()
}

func serveBolt3(conn net.Conn, rows func(string) ([]string, [][]string)) error {
	var handshake [20]byte
	if _, err := io.ReadFull(conn, handshake[:]); err != nil {
		return err
	}
	if !bytes.Equal(handshake[:4], []byte{0x60, 0x60, 0xb0, 0x17}) {
		return fmt.Errorf("invalid handshake")
	}
	if _, err := conn.Write([]byte{0, 0, 0, 3}); err != nil {
		return err
	}
	var records [][]string
	count := false
	for {
		var message []byte
		for {
			var header [2]byte
			if _, err := io.ReadFull(conn, header[:]); err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
			size := binary.BigEndian.Uint16(header[:])
			if size == 0 {
				break
			}
			chunk := make([]byte, size)
			if _, err := io.ReadFull(conn, chunk); err != nil {
				return err
			}
			message = append(message, chunk...)
		}
		if len(message) < 2 {
			return fmt.Errorf("invalid message")
		}
		switch message[1] {
		case 0x01: // HELLO
			if err := boltReply(conn, 0x70, boltMap("server", "Neo4j/3.5.0", "connection_id", "fixture")); err != nil {
				return err
			}
		case 0x11: // BEGIN
			if bytes.Contains(message, []byte("bookmarks")) || bytes.Contains(message, []byte{0x82, 'd', 'b'}) {
				return fmt.Errorf("unexpected bookmark or database metadata")
			}
			if err := boltReply(conn, 0x70, []byte{0xa0}); err != nil {
				return err
			}
		case 0x10: // RUN
			statement, err := boltFirstString(message[2:])
			if err != nil {
				return err
			}
			for _, extension := range []string{"CONSTRAINT", "INDEX", "revision", "dbms.", "CALL "} {
				if strings.Contains(statement, extension) {
					return fmt.Errorf("unexpected Neo4j extension: %s", statement)
				}
			}
			fields := []string{}
			records, count = nil, false
			if strings.Contains(statement, "RETURN count(w)") {
				fields, count = []string{"count"}, true
			} else if rows != nil {
				fields, records = rows(statement)
			}
			metadata := append([]byte{0xa1}, boltString("fields")...)
			metadata = append(metadata, boltStrings(fields)...)
			if err := boltReply(conn, 0x70, metadata); err != nil {
				return err
			}
		case 0x3f: // PULL_ALL
			if count {
				if err := boltReply(conn, 0x71, []byte{0x91, 1}); err != nil {
					return err
				}
			}
			for _, row := range records {
				if err := boltReply(conn, 0x71, boltStrings(row)); err != nil {
					return err
				}
			}
			if err := boltReply(conn, 0x70, []byte{0xa0}); err != nil {
				return err
			}
		case 0x12: // COMMIT: return a bookmark to catch accidental propagation.
			if err := boltReply(conn, 0x70, boltMap("bookmark", "fixture:1")); err != nil {
				return err
			}
		case 0x0f, 0x13: // RESET, ROLLBACK
			if err := boltReply(conn, 0x70, []byte{0xa0}); err != nil {
				return err
			}
		case 0x02: // GOODBYE
			return nil
		default:
			return fmt.Errorf("unexpected message signature %x", message[1])
		}
	}
}

func boltFirstString(data []byte) (string, error) {
	if len(data) == 0 {
		return "", io.ErrUnexpectedEOF
	}
	size, offset := 0, 1
	switch {
	case data[0] >= 0x80 && data[0] <= 0x8f:
		size = int(data[0] & 0xf)
	case data[0] == 0xd0 && len(data) >= 2:
		size, offset = int(data[1]), 2
	case data[0] == 0xd1 && len(data) >= 3:
		size, offset = int(binary.BigEndian.Uint16(data[1:3])), 3
	default:
		return "", fmt.Errorf("unsupported string marker")
	}
	if len(data) < offset+size {
		return "", io.ErrUnexpectedEOF
	}
	return string(data[offset : offset+size]), nil
}

func boltString(value string) []byte {
	if len(value) < 16 {
		return append([]byte{0x80 + byte(len(value))}, value...)
	}
	if len(value) < 256 {
		return append([]byte{0xd0, byte(len(value))}, value...)
	}
	return append([]byte{0xd1, byte(len(value) >> 8), byte(len(value))}, value...)
}

func boltStrings(values []string) []byte {
	out := []byte{0x90 + byte(len(values))}
	for _, value := range values {
		out = append(out, boltString(value)...)
	}
	return out
}

func boltMap(pairs ...string) []byte {
	out := []byte{0xa0 + byte(len(pairs)/2)}
	for _, value := range pairs {
		out = append(out, boltString(value)...)
	}
	return out
}

func boltReply(conn net.Conn, tag byte, value []byte) error {
	message := append([]byte{0xb1, tag}, value...)
	chunk := append([]byte{byte(len(message) >> 8), byte(len(message))}, message...)
	_, err := conn.Write(append(chunk, 0, 0))
	return err
}

func fixtureProvider(t *testing.T, rows func(string) ([]string, [][]string)) *Provider {
	t.Helper()
	p, err := NewProvider(graphstore.ProviderConfig{Options: map[string]string{
		"uri": bolt3Fixture(t, rows), "username": "test", "password": "test-password",
		"dialect": "opencypher", "database": "", "timeout": "5s",
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func TestOpenCypherBolt3Connection(t *testing.T) {
	p := fixtureProvider(t, nil)
	ctx := context.Background()
	if err := p.OpenWorkspace(ctx, model.WorkspaceMetadata{ID: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := p.EnsureSchema(ctx, "test"); err != nil {
		t.Fatal(err)
	}
	health, err := p.Health(ctx)
	if err != nil || health.Provider != "neo4j" || health.Status != "ok" || !strings.Contains(health.Message, "one provider instance") {
		t.Fatalf("unexpected health: %+v %v", health, err)
	}
}

func TestOpenCypherRejectsDuplicateRecords(t *testing.T) {
	p := fixtureProvider(t, func(statement string) ([]string, [][]string) {
		if strings.Contains(statement, "RETURN n.key AS key") {
			return []string{"key", "data"}, [][]string{{"duplicate", "{}"}, {"duplicate", "{}"}}
		}
		return nil, nil
	})
	_, err := p.QueryEntities(context.Background(), model.EntityQueryPlan{Workspace: "test"})
	if err == nil || !strings.Contains(err.Error(), "duplicate graph record") {
		t.Fatalf("expected duplicate rejection: %v", err)
	}
}

func TestOpenCypherWriteGateHonorsCancellation(t *testing.T) {
	p := fixtureProvider(t, nil)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		_, err := p.transaction(context.Background(), true, func(ctx context.Context, tx driver.ManagedTransaction) (any, error) {
			close(entered)
			<-release
			return nil, consume(ctx, tx, "RETURN 1", nil)
		})
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("first writer did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := p.OpenWorkspace(ctx, model.WorkspaceMetadata{ID: "test"})
	close(release)
	if firstErr := <-done; firstErr != nil {
		t.Fatal(firstErr)
	}
	if err != context.DeadlineExceeded {
		t.Fatalf("writer bypassed serialization: %v", err)
	}
	// A canceled waiter must not leak the gate.
	if err := p.OpenWorkspace(context.Background(), model.WorkspaceMetadata{ID: "test"}); err != nil {
		t.Fatal(err)
	}
}
