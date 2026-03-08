package starters

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"ems-bridge/messages"
)

// RestStarter registers an HTTP handler on the application mux and forwards
// each matching request as a Message to the route handler.
type RestStarter struct {
	StarterConfig
	method  string
	uri     string
	handler Handler
}

func newRestStarter(s StarterConfig, handler Handler) (*RestStarter, error) {
	if s.Mux == nil {
		return nil, fmt.Errorf("starter %q: RestStarter requires an HTTP mux (none provided)", s.ID)
	}

	method := strings.ToUpper(s.Properties["method"])
	if method == "" {
		return nil, fmt.Errorf("starter %q: missing required property \"method\"", s.ID)
	}

	uri := s.Properties["uri"]
	if uri == "" {
		return nil, fmt.Errorf("starter %q: missing required property \"uri\"", s.ID)
	}

	return &RestStarter{
		StarterConfig: s,
		method:        method,
		uri:           uri,
		handler:       handler,
	}, nil
}

func (s *RestStarter) Start() error {
	slog.Info("RestStarter: registering handler", "id", s.ID, "method", s.method, "uri", s.uri)
	s.Mux.HandleFunc(s.uri, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != s.method {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("RestStarter: reading request body", "id", s.ID, "err", err)
			http.Error(w, "failed to read request body", http.StatusInternalServerError)
			return
		}

		headers := make(map[string]string, len(r.Header)+2)
		for k, vals := range r.Header {
			headers[k] = strings.Join(vals, ",")
		}
		headers["method"] = r.Method
		headers["uri"] = r.RequestURI

		msg := messages.NewMessage(body, headers, map[string]any{})
		slog.Info("RestStarter: message created", "id", s.ID, "message", msg)
		msg.Print()

		if err := s.handler(msg); err != nil {
			slog.Error("RestStarter: handler error", "id", s.ID, "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var respBody []byte
		switch p := msg.Payload.(type) {
		case []byte:
			respBody = p
		case string:
			respBody = []byte(p)
		case nil:
			// no body
		default:
			data, err := json.Marshal(p)
			if err != nil {
				slog.Error("RestStarter: marshalling payload", "id", s.ID, "err", err)
				http.Error(w, "failed to marshal response payload", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			respBody = data
		}
		w.WriteHeader(http.StatusOK)
		if len(respBody) > 0 {
			w.Write(respBody) //nolint:errcheck
		}
	})
	return nil
}

func (s *RestStarter) Stop() error {
	slog.Info("RestStarter stopped", "id", s.ID)
	return nil
}
