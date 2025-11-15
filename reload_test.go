package reload_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/johanbrandhorst/reload"
	"github.com/neilotoole/slogt"
)

func TestReloader(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})

	dir := t.TempDir()
	r, err := reload.NewMiddleware(testHandler, dir, slogt.New(t))
	if err != nil {
		t.Fatalf("Failed to create reloader: %v", err)
	}
	t.Cleanup(func() {
		err := r.Close()
		if err != nil {
			t.Errorf("Failed to close reloader: %v", err)
		}
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("Failed to make GET request: %v", err)
	}
	t.Cleanup(func() {
		err := resp.Body.Close()
		if err != nil {
			t.Errorf("Failed to close response body: %v", err)
		}
	})

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	if string(body) != "Hello, World!" {
		t.Errorf("Expected body 'Hello, World!', got %s", string(body))
	}

	wg := &sync.WaitGroup{}
	wg.Add(5)
	startCh := make(chan struct{})
	for range 5 {
		go func() {
			defer wg.Done()
			<-startCh
			conn, _, err := websocket.Dial(t.Context(), srv.URL+"/watch", &websocket.DialOptions{HTTPClient: srv.Client()})
			if err != nil {
				t.Errorf("Failed to dial WebSocket: %v", err)
				return
			}
			t.Cleanup(func() {
				err := conn.Close(websocket.StatusNormalClosure, "test complete")
				if err != nil {
					t.Errorf("Failed to close WebSocket connection: %v", err)
				}
			})

			doneCh := make(chan struct{})
			go func() {
				defer close(doneCh)
				// Blocks until a msg is available
				msgTyp, msg, err := conn.Read(t.Context())
				if err != nil {
					t.Errorf("Failed to read WebSocket message: %v", err)
					return
				}
				if msgTyp != websocket.MessageText {
					t.Errorf("Expected message type Text, got %d", msgTyp)
					return
				}
				if string(msg) != "reload" {
					t.Errorf("Expected message 'reload', got %s", string(msg))
					return
				}
			}()

			// Wait for a bit to ensure the goroutine is running and blocked
			select {
			case <-doneCh:
				t.Error("Done channel should not be closed yet")
				return
			case <-t.Context().Done():
				t.Error("Test context timed out waiting for goroutine start")
				return
			case <-time.After(10 * time.Millisecond):
			}

			// Trigger reload
			err = os.WriteFile(filepath.Join(dir, "test.txt"), []byte("anything"), 0644)
			if err != nil {
				t.Errorf("Failed to write file: %v", err)
				return
			}

			// Wait for the goroutine to finish
			select {
			case <-doneCh:
			case <-t.Context().Done():
				t.Error("Test context timed out waiting for goroutine to finish")
				return
			case <-time.After(time.Second):
				t.Error("Test timed out waiting for goroutine to finish")
				return
			}

		}()
	}
	close(startCh)
	wg.Wait()

}
