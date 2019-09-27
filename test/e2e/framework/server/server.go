package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"sync"
	"text/template"

	"github.com/digitalocean/godo/test/e2e/framework"
	"github.com/gorilla/mux"
)

type Error struct {
	Error string `json:"error"`
}

type Server struct {
	handler http.Handler

	templatesPath string
	templates     map[string]*template.Template

	results map[string]*framework.Result
	mu      sync.RWMutex
}

func New(templatesPath string) *Server {
	server := &Server{
		templatesPath: templatesPath,
		results:       make(map[string]*framework.Result),
	}

	err := server.loadTemplates()
	if err != nil {
		panic("failed to load templates")
	}

	r := mux.NewRouter()
	r.HandleFunc("/", LogHandlerFunc(server.listResults)).Methods(http.MethodGet)
	r.HandleFunc("/api/results/{result_id}", LogHandlerFunc(server.saveResult)).Methods(http.MethodPost)
	r.HandleFunc("/api/results/{result_id}/logs", LogHandlerFunc(server.saveResultLogs)).Methods(http.MethodPost)
	server.handler = r

	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) listResults(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sortedResults []*framework.Result
	for _, result := range s.results {
		sortedResults = append(sortedResults, result)
	}
	sort.Slice(sortedResults, func(a, b int) bool {
		return sortedResults[a].RanAt.After(sortedResults[b].RanAt)
	})

	value := &struct {
		Results []*framework.Result
	}{
		Results: sortedResults,
	}

	s.render(w, r, "results_index", value)
}

func (s *Server) saveResult(w http.ResponseWriter, r *http.Request) {
	var result framework.Result
	err := json.NewDecoder(r.Body).Decode(&result)
	if err != nil {
		s.renderJSONError(w, fmt.Errorf("invalid result: %s", err), 400)
		return
	}

	// NOTE(nan) this is a quirk with how go test works. Each package is actually
	// run separately. This means the test context for each package will only have
	// the results for that package. We need to merge them here.
	func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		existingResult, exists := s.results[result.ID]
		if !exists {
			s.results[result.ID] = &result
			return
		}
		for name, test := range result.Tests {
			existingResult.Tests[name] = test
		}
	}()

	defer s.cleanupResults()

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) saveResultLogs(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	vars := mux.Vars(r)
	result, exists := s.results[vars["result_id"]]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	logData, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	result.Logs = logData

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, value interface{}) {
	var b bytes.Buffer
	if err := s.ExecuteTemplate(name, &b, value); err != nil {
		s.renderError(w, r, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	b.WriteTo(w)
}

func (s *Server) renderError(w http.ResponseWriter, r *http.Request, err error, status int) {
	value := struct {
		Status int
		Error  error
	}{
		Status: status,
		Error:  err,
	}

	var b bytes.Buffer
	if err := s.ExecuteTemplate("error", &b, value); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%+v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	b.WriteTo(w)
}

func (s *Server) renderJSONError(w http.ResponseWriter, err error, status int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	resp := &Error{
		Error: err.Error(),
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) cleanupResults() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.results) < 25 {
		return
	}

	var sortedResults []*framework.Result
	for _, result := range s.results {
		sortedResults = append(sortedResults, result)
	}
	sort.Slice(sortedResults, func(a, b int) bool {
		return sortedResults[a].RanAt.After(sortedResults[b].RanAt)
	})
	sortedResults = sortedResults[:25]

	keepIDs := make(map[string]struct{})
	for _, result := range sortedResults {
		keepIDs[result.ID] = struct{}{}
	}

	for id := range s.results {
		if _, keep := keepIDs[id]; !keep {
			delete(s.results, id)
		}
	}
}
