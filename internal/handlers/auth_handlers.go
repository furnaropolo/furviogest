package handlers

import (
	"fmt"
	"furviogest/internal/auth"
	"furviogest/internal/middleware"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var templates map[string]*template.Template

// InitTemplates inizializza i template parsando ogni file con base.html
func InitTemplates(templatesDir string) error {
	templates = make(map[string]*template.Template)

	baseFile := filepath.Join(templatesDir, "base.html")

	// Leggi tutti i file html nella directory
	files, err := os.ReadDir(templatesDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".html") || file.Name() == "base.html" {
			continue
		}

		templateFile := filepath.Join(templatesDir, file.Name())
		funcMap := template.FuncMap{
			"add": func(a, b int) int { return a + b },
			"sub": func(a, b int) int { return a - b },
			"mod": func(a, b int) int { return a % b },
			"lower": strings.ToLower,
			"slugify": func(s string) string {
				return strings.ReplaceAll(strings.ToLower(s), " ", "-")
			},
			"seq": func(start, end int) []int {
				var result []int
				for i := start; i <= end; i++ {
					result = append(result, i)
				}
				return result
			},
			"euro": func(f float64) string { return strings.Replace(fmt.Sprintf("%.2f €", f), ".", ",", 1) },
			"divFloat": func(a int64, b int64) float64 { return float64(a) / float64(b) },
		}
		tmpl, err := template.New(file.Name()).Funcs(funcMap).ParseFiles(baseFile, templateFile)
		if err != nil {
			return err
		}
		templates[file.Name()] = tmpl
	}

	return nil
}

// PageData contiene i dati comuni per tutte le pagine
type PageData struct {
	Title       string
	Session     *auth.Session
	Error       string
	Success     string
	Data        interface{}
	CurrentYear int
}

// renderTemplate esegue il rendering di un template
func renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl, ok := templates[name]
	if !ok {
		http.Error(w, "Template non trovato: "+name, http.StatusInternalServerError)
		return
	}
	err := tmpl.ExecuteTemplate(w, "base", data)
	if err != nil {
		log.Printf("[TEMPLATE ERROR] %s: %v", name, err)
	}
}

// NewPageData crea un nuovo PageData con i dati comuni
func NewPageData(title string, r *http.Request) PageData {
	return PageData{
		Title:       title,
		Session:     middleware.GetSession(r),
		CurrentYear: time.Now().Year(),
	}
}

// LoginPage mostra la pagina di login
func LoginPage(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:       "Login - FurvioGest",
		CurrentYear: time.Now().Year(),
	}

	if r.Method == http.MethodGet {
		renderTemplate(w, "login.html", data)
		return
	}

	// POST - gestisce il login
	username := r.FormValue("username")
	password := r.FormValue("password")

	session, err := auth.Login(username, password)
	if err != nil {
		data.Error = "Credenziali non valide"
		renderTemplate(w, "login.html", data)
		return
	}

	// Imposta il cookie di sessione
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    session.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Mettere true in produzione con HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 ore
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout esegue il logout
func Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		auth.Logout(cookie.Value)
	}

	// Rimuovi il cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "session_token",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// Dashboard mostra la dashboard principale
func Dashboard(w http.ResponseWriter, r *http.Request) {
	// Route "/" è un catch-all in ServeMux, controlliamo se è la root esatta
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data := NewPageData("Dashboard - FurvioGest", r)

	// Mostra avviso backup solo per tecnici
	if data.Session != nil && data.Session.IsTecnico() {
		erroreBackup := GetUltimoBackupErrore()
		if erroreBackup != "" {
			data.Data = map[string]interface{}{
				"ErroreBackup": erroreBackup,
			}
		}
	}

	renderTemplate(w, "dashboard.html", data)
}

// CambioPassword gestisce il cambio password
func CambioPassword(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Cambio Password - FurvioGest", r)

	if r.Method == http.MethodGet {
		renderTemplate(w, "cambio_password.html", data)
		return
	}

	// POST
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	passwordAttuale := r.FormValue("password_attuale")
	nuovaPassword := r.FormValue("nuova_password")
	confermaPassword := r.FormValue("conferma_password")

	// Verifica password attuale
	_, err := auth.Login(session.Username, passwordAttuale)
	if err != nil {
		data.Error = "Password attuale non corretta"
		renderTemplate(w, "cambio_password.html", data)
		return
	}

	// Verifica che le nuove password coincidano
	if nuovaPassword != confermaPassword {
		data.Error = "Le nuove password non coincidono"
		renderTemplate(w, "cambio_password.html", data)
		return
	}

	// Verifica lunghezza minima
	if len(nuovaPassword) < 6 {
		data.Error = "La password deve essere di almeno 6 caratteri"
		renderTemplate(w, "cambio_password.html", data)
		return
	}

	// Aggiorna la password
	err = auth.UpdatePassword(session.UserID, nuovaPassword)
	if err != nil {
		data.Error = "Errore durante l'aggiornamento della password"
		renderTemplate(w, "cambio_password.html", data)
		return
	}

	data.Success = "Password aggiornata con successo"
	renderTemplate(w, "cambio_password.html", data)
}
