package assistant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	valid := Request{Mode: "explain", Phrase: "c'est la vie", Language: "English", Consent: true}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Request){
		func(r *Request) { r.Mode = "chat" }, func(r *Request) { r.Phrase = " " },
		func(r *Request) { r.Phrase = strings.Repeat("é", 1201) }, func(r *Request) { r.Language = "" },
		func(r *Request) { r.Language = strings.Repeat("x", 41) }, func(r *Request) { r.Consent = false },
	} {
		r := valid
		mutate(&r)
		if r.Validate() == nil {
			t.Fatalf("accepted invalid request: %+v", r)
		}
	}
}

func TestOpenRouterBoundaries(t *testing.T) {
	for _, mode := range []string{"explain", "translate"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-key" {
					t.Error("missing server credential")
				}
				var body struct {
					Model     string              `json:"model"`
					MaxTokens int                 `json:"max_tokens"`
					Provider  map[string]any      `json:"provider"`
					Messages  []map[string]string `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.Model != "google/gemini-3.8-flash" || body.MaxTokens != 800 || len(body.Messages) != 2 {
					t.Error("unbounded or wrong provider payload")
				}
				if body.Provider["data_collection"] != "deny" || body.Provider["zdr"] != true {
					t.Error("missing privacy constraints")
				}
				if body.Messages[1]["content"] != `{"output_language":"English","phrase":"bonjour"}` {
					t.Error("sent more than explicit phrase/language")
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"choices":[{"message":{"content":"Hello."}}]}`))
			}))
			defer server.Close()
			client := New("test-key", "google/gemini-3.8-flash")
			client.endpoint = server.URL
			answer, err := client.Ask(context.Background(), Request{Mode: mode, Phrase: "bonjour", Language: "English", Consent: true})
			if err != nil || answer != "Hello." {
				t.Fatalf("%q %v", answer, err)
			}
		})
	}
}

func TestProviderFailureDoesNotLeak(t *testing.T) {
	for _, status := range []int{401, 429, 500} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte("private phrase and secret-key"))
		}))
		client := New("secret-key", "google/gemini-3.8-flash")
		client.endpoint = server.URL
		_, err := client.Ask(context.Background(), Request{Mode: "explain", Phrase: "private phrase", Language: "English", Consent: true})
		server.Close()
		if err == nil || strings.Contains(err.Error(), "private phrase") || strings.Contains(err.Error(), "secret-key") {
			t.Fatal("provider details leaked")
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"choices":[]}`)) }))
	defer server.Close()
	client := New("test-key", "test-model")
	client.endpoint = server.URL
	if _, err := client.Ask(context.Background(), Request{Mode: "translate", Phrase: "hi", Language: "French", Consent: true}); err == nil {
		t.Fatal("empty answer accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Ask(ctx, Request{Mode: "translate", Phrase: "hi", Language: "French", Consent: true}); err == nil {
		t.Fatal("cancelled request continued")
	}
}
