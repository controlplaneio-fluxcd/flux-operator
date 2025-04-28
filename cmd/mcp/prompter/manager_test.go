// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package prompter

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManager_FetchMarkdown(t *testing.T) {
	tests := []struct {
		name           string
		serverHandler  http.HandlerFunc
		expectedBody   string
		expectedError  string
		expectedStatus int
	}{
		{
			name: "valid markdown response",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("# Markdown Content"))
			},
			expectedBody:   "# Markdown Content",
			expectedError:  "",
			expectedStatus: http.StatusOK,
		},
		{
			name: "non-200 status code",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("Not Found"))
			},
			expectedBody:   "",
			expectedError:  "unexpected status code: 404",
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "server unavailable",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedBody:   "",
			expectedError:  "unexpected status code: 500",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:          "empty URL",
			serverHandler: nil,
			expectedBody:  "",
			expectedError: "Get \"\": unsupported protocol scheme \"\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.serverHandler != nil {
				server = httptest.NewServer(tt.serverHandler)
				defer server.Close()
			}

			url := ""
			if server != nil {
				url = server.URL
			}

			manager := &Manager{}
			body, err := manager.fetchMarkdown(url)

			if tt.expectedBody != body {
				t.Errorf("expected body: %q, got: %q", tt.expectedBody, body)
			}

			if err != nil && tt.expectedError == "" {
				t.Errorf("unexpected error: %v", err)
			} else if err == nil && tt.expectedError != "" {
				t.Errorf("expected error: %q, got: nil", tt.expectedError)
			} else if err != nil && err.Error() != tt.expectedError {
				t.Errorf("expected error: %q, got: %q", tt.expectedError, err.Error())
			}
		})
	}
}
