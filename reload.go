package reload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/coder/websocket"
	"github.com/fsnotify/fsnotify"
	"github.com/teivah/broadcast"
)

type autoReloader struct {
	handler http.Handler
	relay   *broadcast.Relay[struct{}]
	log     *slog.Logger
	cancel  context.CancelCauseFunc
	ctx     context.Context
	wg      sync.WaitGroup
}

func NewMiddleware(next http.Handler, root string, log *slog.Logger) (*autoReloader, error) {
	w, err := createWatcher(root)
	if err != nil {
		return nil, err
	}
	relay := broadcast.NewRelay[struct{}]()
	ctx, cancel := context.WithCancelCause(context.Background())

	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	r := &autoReloader{
		handler: next,
		relay:   relay,
		log:     log,
		ctx:     ctx,
		cancel:  cancel,
	}

	r.wg.Go(func() {
		watch(ctx, w, relay, log)
	})

	return r, nil
}

func createWatcher(root string) (*fsnotify.Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create filesystem watcher: %w", err)
	}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk directory: %w", err)
		}
		if d.IsDir() {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("failed to get absolute path for %q: %w", path, err)
			}
			err = w.Add(absPath)
			if err != nil {
				return fmt.Errorf("failed to add directory to watcher: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}
	return w, nil
}

func watch(ctx context.Context, w *fsnotify.Watcher, relay *broadcast.Relay[struct{}], log *slog.Logger) {
	defer w.Close()
	defer relay.Close()
	defer log.Info("Stopping file watcher")
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
				log.Info("File modified", "file", event.Name)
				relay.Broadcast(struct{}{})
			}
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			log.Error("Error watching file", "error", err)
		}
	}
}

func (r *autoReloader) Close() error {
	if r.cancel != nil {
		r.cancel(errors.New("reloader closed"))
	}
	r.wg.Wait()
	return nil
}

func (r *autoReloader) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/watch" {
		ws, err := websocket.Accept(w, req, nil)
		if err != nil {
			r.log.Error("Failed to accept websocket connection", "error", err)
			return
		}
		l := r.relay.Listener(0)
		r.log.Info("WebSocket connection established", "remoteAddr", req.RemoteAddr)
		go func() {
			defer r.log.Info("WebSocket connection closed", "remoteAddr", req.RemoteAddr)
			defer l.Close()
			// Wait for notification to come in
			select {
			case <-l.Ch():
				r.log.Info("Received reload notification", "remoteAddr", req.RemoteAddr)
			case <-r.ctx.Done():
				r.log.Info("Reloader closed, closing WebSocket connection", "remoteAddr", req.RemoteAddr)
				return
			}
			// Send reload message to the client
			r.log.Info("Sending reload message to client", "remoteAddr", req.RemoteAddr)
			err := ws.Write(r.ctx, websocket.MessageText, []byte("reload"))
			if err != nil {
				r.log.Error("Failed to send reload message", "error", err)
			}
			// We're done with this request since the browser will reload
			ws.Close(websocket.StatusNormalClosure, "Reloading")
		}()
		return
	}
	// Ensure no caching of responses
	w.Header().Set("Cache-Control", "no-cache")
	r.handler.ServeHTTP(w, req)
}
