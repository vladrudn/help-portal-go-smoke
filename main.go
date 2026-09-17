package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
)

type section struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Body    string `json:"body"`
}

type createSectionInput struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type store struct {
	mu       sync.RWMutex
	sections []section
	nextID   int
}

func newStore() *store {
	return &store{nextID: 4, sections: []section{
		{ID: 1, Title: "Перший документ", Summary: "Шість коротких кроків для створення документа в СЕДО.", Body: "Оберіть тип документа, заповніть реквізити та збережіть чернетку."},
		{ID: 2, Title: "Внутрішній документообіг", Summary: "Маршрути, погодження та контроль виконання.", Body: "Перед надсиланням перевірте адресатів і маршрут погодження."},
		{ID: 3, Title: "Вихідні документи", Summary: "Підготовка, реєстрація і відправлення вихідного листа.", Body: "Після реєстрації документ отримує номер і стає доступним у журналі."},
	}}
}

func (s *store) list(query string) []section {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query = strings.ToLower(strings.TrimSpace(query))
	result := make([]section, 0, len(s.sections))
	for _, item := range s.sections {
		if query == "" || strings.Contains(strings.ToLower(item.Title+" "+item.Summary+" "+item.Body), query) {
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (s *store) add(input createSectionInput) (section, bool) {
	input.Title = strings.TrimSpace(input.Title)
	input.Summary = strings.TrimSpace(input.Summary)
	if input.Title == "" || input.Summary == "" {
		return section{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item := section{ID: s.nextID, Title: input.Title, Summary: input.Summary, Body: "Тестовий розділ створено в пам'яті. Після перезапуску він зникне."}
	s.nextID++
	s.sections = append(s.sections, item)
	return item, true
}

func main() {
	data := newStore()
	r := chi.NewRouter()
	r.Use(securityHeaders)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/api/sections", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, data.list(r.URL.Query().Get("q")))
	})
	r.Post("/api/sections", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var input createSectionInput
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Некоректні дані форми."})
			return
		}
		item, ok := data.add(input)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Заповніть назву й короткий опис."})
			return
		}
		writeJSON(w, http.StatusCreated, item)
	})

	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "web/dist"
	}
	r.Handle("/*", spa(staticDir))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	log.Printf("help-portal smoke test listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func spa(staticDir string) http.Handler {
	files := http.FileServer(http.Dir(staticDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Clean(r.URL.Path)
		if name != "." && name != "/" {
			if info, err := os.Stat(filepath.Join(staticDir, name)); err == nil && !info.IsDir() {
				files.ServeHTTP(w, r)
				return
			}
		}
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})
}
