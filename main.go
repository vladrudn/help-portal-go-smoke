package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

const sessionCookie = "help_portal_session"

type section struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Body    string `json:"body"`
	Order   int    `json:"-"`
}
type sectionInput struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Body    string `json:"body"`
}
type store struct {
	mu       sync.RWMutex
	sections []section
}
type supabase struct {
	url, key string
	client   *http.Client
}
type authToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}
type authUser struct {
	ID string `json:"id"`
}
type profile struct {
	Role   string `json:"role"`
	Active bool   `json:"is_active"`
}
type remoteSection struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Order   int    `json:"order_number"`
}

func newStore() *store {
	files := []struct{ title, filename, summary string }{{"Вступ до СЕДО", "01-vstup-do-sedo.md", "Базова навігація та перші дії в системі."}, {"Внутрішній документообіг", "02-vnutrishnij-dokumentoobig.md", "Маршрути, погодження і контроль внутрішніх документів."}, {"Вихідні документи", "03-vyhidni-dokumenty.md", "Підготовка та реєстрація вихідних документів."}}
	items := make([]section, 0, len(files))
	for i, f := range files {
		body, err := os.ReadFile(filepath.Join("content", f.filename))
		if err != nil {
			log.Printf("read local content %s: %v", f.filename, err)
		}
		items = append(items, section{ID: fmt.Sprintf("local-%d", i+1), Title: f.title, Summary: f.summary, Body: string(body), Order: i + 1})
	}
	return &store{sections: items}
}
func (s *store) list(query string) []section {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return filterSections(s.sections, query)
}
func filterSections(items []section, query string) []section {
	query = strings.ToLower(strings.TrimSpace(query))
	result := make([]section, 0, len(items))
	for _, item := range items {
		if query == "" || strings.Contains(strings.ToLower(item.Title+" "+item.Summary+" "+item.Body), query) {
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Order < result[j].Order })
	return result
}
func summary(body string) string {
	text := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(body, "#", ""), "\n", " "))
	text = strings.Join(strings.Fields(text), " ")
	if len([]rune(text)) > 130 {
		return string([]rune(text)[:130]) + "…"
	}
	return text
}
func validInput(in sectionInput) (sectionInput, bool) {
	in.Title = strings.TrimSpace(in.Title)
	in.Summary = strings.TrimSpace(in.Summary)
	in.Body = strings.TrimSpace(in.Body)
	return in, in.Title != "" && in.Summary != "" && in.Body != ""
}
func newSupabase() *supabase {
	u, k := strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"), os.Getenv("SUPABASE_ANON_KEY")
	if u == "" || k == "" {
		return nil
	}
	return &supabase{url: u, key: k, client: &http.Client{Timeout: 10 * time.Second}}
}
func (s *supabase) request(method, path, token string, input any, out any) error {
	var body io.Reader
	if input != nil {
		raw, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, s.url+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("apikey", s.key)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if out != nil {
		req.Header.Set("Accept", "application/json")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("supabase status %d", resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
func (s *supabase) sections(token, query string) ([]section, error) {
	var raw []remoteSection
	if err := s.request(http.MethodGet, "/rest/v1/sections?select=id,title,content,order_number&order=order_number.asc,id.asc", token, nil, &raw); err != nil {
		return nil, err
	}
	got := make([]section, 0, len(raw))
	for _, r := range raw {
		got = append(got, section{ID: r.ID, Title: r.Title, Summary: summary(r.Content), Body: r.Content, Order: r.Order})
	}
	return filterSections(got, query), nil
}
func (s *supabase) login(login, password string) (authToken, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	if len(login) < 3 || len(login) > 32 {
		return authToken{}, errors.New("Невірний логін або пароль.")
	}
	var token authToken
	err := s.request(http.MethodPost, "/auth/v1/token?grant_type=password", "", map[string]string{"email": login + "@help-portal.local", "password": password}, &token)
	if err != nil {
		return authToken{}, errors.New("Невірний логін або пароль.")
	}
	if _, err = s.admin(token.AccessToken); err != nil {
		return authToken{}, err
	}
	return token, nil
}
func (s *supabase) admin(token string) (authUser, error) {
	var user authUser
	if err := s.request(http.MethodGet, "/auth/v1/user", token, nil, &user); err != nil {
		return user, errors.New("Потрібен вхід адміністратора.")
	}
	var rows []profile
	if err := s.request(http.MethodGet, "/rest/v1/profiles?select=role,is_active&id=eq."+url.QueryEscape(user.ID), token, nil, &rows); err != nil || len(rows) != 1 || rows[0].Role != "admin" || !rows[0].Active {
		return user, errors.New("Доступ дозволено лише адміністратору.")
	}
	return user, nil
}
func (s *supabase) save(token, id string, in sectionInput) (section, error) {
	in, ok := validInput(in)
	if !ok {
		return section{}, errors.New("Заповніть назву, опис і текст інструкції.")
	}
	payload := map[string]any{"title": in.Title, "content": in.Body}
	method, path := http.MethodPost, "/rest/v1/sections"
	if id != "" {
		method = http.MethodPatch
		path = "/rest/v1/sections?id=eq." + url.QueryEscape(id)
	} else {
		all, err := s.sections(token, "")
		if err != nil {
			return section{}, err
		}
		payload["order_number"] = len(all) + 1
	}
	var rows []remoteSection
	if err := s.request(method, path, token, payload, &rows); err != nil {
		return section{}, err
	}
	if len(rows) != 1 {
		return section{}, errors.New("Розділ не збережено.")
	}
	r := rows[0]
	return section{ID: r.ID, Title: r.Title, Summary: summary(r.Content), Body: r.Content, Order: r.Order}, nil
}
func (s *supabase) remove(token, id string) error {
	return s.request(http.MethodDelete, "/rest/v1/sections?id=eq."+url.QueryEscape(id), token, nil, nil)
}

func main() {
	local, remote := newStore(), newSupabase()
	r := chi.NewRouter()
	r.Use(securityHeaders)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/api/sections", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if remote == nil {
			writeJSON(w, http.StatusOK, local.list(q))
			return
		}
		items, err := remote.sections("", q)
		if err != nil {
			log.Printf("public sections: %v", err)
			writeJSON(w, http.StatusOK, local.list(q))
			return
		}
		if len(items) == 0 {
			items = local.list(q)
		}
		writeJSON(w, http.StatusOK, items)
	})
	r.Get("/api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		if remote == nil {
			writeJSON(w, http.StatusOK, map[string]bool{"admin": false})
			return
		}
		_, err := remote.admin(cookieToken(r))
		writeJSON(w, http.StatusOK, map[string]bool{"admin": err == nil})
	})
	r.Post("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if remote == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Авторизація ще не налаштована."})
			return
		}
		var in struct {
			Login    string `json:"login"`
			Password string `json:"password"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&in) != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Некоректні дані входу."})
			return
		}
		token, err := remote.login(in.Login, in.Password)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token.AccessToken, Path: "/", HttpOnly: true, Secure: os.Getenv("COOKIE_SECURE") != "false", SameSite: http.SameSiteLaxMode, MaxAge: token.ExpiresIn})
		writeJSON(w, http.StatusOK, map[string]bool{"admin": true})
	})
	r.Post("/api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: os.Getenv("COOKIE_SECURE") != "false", MaxAge: -1})
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	r.Post("/api/sections", saveHandler(remote, ""))
	r.Put("/api/sections/{id}", func(w http.ResponseWriter, r *http.Request) { saveHandler(remote, chi.URLParam(r, "id"))(w, r) })
	r.Delete("/api/sections/{id}", func(w http.ResponseWriter, r *http.Request) {
		if remote == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Редактор ще не налаштовано."})
			return
		}
		token := cookieToken(r)
		if _, err := remote.admin(token); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		if err := remote.remove(token, chi.URLParam(r, "id")); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Не вдалося видалити інструкцію."})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
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
	log.Printf("help portal listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
func saveHandler(remote *supabase, id string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if remote == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Редактор ще не налаштовано."})
			return
		}
		token := cookieToken(r)
		if _, err := remote.admin(token); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		var in sectionInput
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&in) != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Некоректні дані форми."})
			return
		}
		item, err := remote.save(token, id, in)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Не вдалося зберегти зміни."})
			return
		}
		status := http.StatusOK
		if id == "" {
			status = http.StatusCreated
		}
		writeJSON(w, status, item)
	}
}
func cookieToken(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
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
		w.Header().Set("Referrer-Policy", "same-origin")
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
