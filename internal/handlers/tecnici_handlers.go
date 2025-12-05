package handlers

import (
	"database/sql"
	"fmt"
	"furviogest/internal/auth"
	"furviogest/internal/database"
	"furviogest/internal/middleware"
	"furviogest/internal/models"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ListaTecnici mostra la lista dei tecnici
func ListaTecnici(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Tecnici - FurvioGest", r)

	rows, err := database.DB.Query(`
		SELECT id, username, nome, cognome, email, telefono, ruolo, attivo, documento_path, created_at
		FROM utenti ORDER BY cognome, nome
	`)
	if err != nil {
		data.Error = "Errore nel recupero dei tecnici"
		renderTemplate(w, "tecnici_lista.html", data)
		return
	}
	defer rows.Close()

	var tecnici []models.Utente
	for rows.Next() {
		var t models.Utente
		var docPath sql.NullString
		err := rows.Scan(&t.ID, &t.Username, &t.Nome, &t.Cognome, &t.Email, &t.Telefono, &t.Ruolo, &t.Attivo, &docPath, &t.CreatedAt)
		if err != nil {
			continue
		}
		if docPath.Valid {
			t.DocumentoPath = docPath.String
		}
		tecnici = append(tecnici, t)
	}

	data.Data = tecnici
	renderTemplate(w, "tecnici_lista.html", data)
}

// NuovoTecnico gestisce la creazione di un nuovo tecnico
func NuovoTecnico(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Nuovo Tecnico - FurvioGest", r)

	if r.Method == http.MethodGet {
		renderTemplate(w, "tecnici_form.html", data)
		return
	}

	// POST - salva il nuovo tecnico
	if err := r.ParseMultipartForm(10 << 20); err != nil { // Max 10MB
		r.ParseForm()
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	nome := strings.TrimSpace(r.FormValue("nome"))
	cognome := strings.TrimSpace(r.FormValue("cognome"))
	email := strings.TrimSpace(r.FormValue("email"))
	telefono := strings.TrimSpace(r.FormValue("telefono"))
	ruolo := models.Ruolo(r.FormValue("ruolo"))

	// Validazione
	if username == "" || password == "" || nome == "" || cognome == "" || email == "" {
		data.Error = "Compila tutti i campi obbligatori"
		renderTemplate(w, "tecnici_form.html", data)
		return
	}

	if ruolo != models.RuoloTecnico && ruolo != models.RuoloGuest {
		ruolo = models.RuoloGuest
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		data.Error = "Errore durante la creazione dell'utente"
		renderTemplate(w, "tecnici_form.html", data)
		return
	}

	// Gestione upload documento
	var documentoPath string
	file, header, err := r.FormFile("documento")
	if err == nil {
		defer file.Close()

		// Crea directory se non esiste
		uploadDir := filepath.Join("web", "static", "uploads", "documenti")
		os.MkdirAll(uploadDir, 0755)

		// Genera nome file unico
		ext := filepath.Ext(header.Filename)
		newFilename := fmt.Sprintf("doc_%s_%d%s", username, time.Now().Unix(), ext)
		filePath := filepath.Join(uploadDir, newFilename)

		// Salva il file
		dst, err := os.Create(filePath)
		if err == nil {
			defer dst.Close()
			io.Copy(dst, file)
			documentoPath = filepath.Join("uploads", "documenti", newFilename)
		}
	}

	// Inserisci nel database
	_, err = database.DB.Exec(`
		INSERT INTO utenti (username, password, nome, cognome, email, telefono, ruolo, attivo, documento_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)
	`, username, hashedPassword, nome, cognome, email, telefono, ruolo, documentoPath)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			data.Error = "Username già esistente"
		} else {
			data.Error = "Errore durante il salvataggio"
		}
		renderTemplate(w, "tecnici_form.html", data)
		return
	}

	http.Redirect(w, r, "/tecnici?success=creato", http.StatusSeeOther)
}

// ModificaTecnico gestisce la modifica di un tecnico esistente
func ModificaTecnico(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Modifica Tecnico - FurvioGest", r)

	// Estrai ID dall'URL
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/tecnici", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/tecnici", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		var t models.Utente
		var docPath sql.NullString
		var telefono sql.NullString
		err := database.DB.QueryRow(`
			SELECT id, username, nome, cognome, email, telefono, ruolo, attivo, documento_path,
			COALESCE(smtp_server, ''), COALESCE(smtp_port, 587), COALESCE(smtp_user, ''), COALESCE(smtp_password, '')
			FROM utenti WHERE id = ?
		`, id).Scan(&t.ID, &t.Username, &t.Nome, &t.Cognome, &t.Email, &telefono, &t.Ruolo, &t.Attivo, &docPath,
			&t.SMTPServer, &t.SMTPPort, &t.SMTPUser, &t.SMTPPassword)

		if err != nil {
			http.Redirect(w, r, "/tecnici", http.StatusSeeOther)
			return
		}
		if docPath.Valid {
			t.DocumentoPath = docPath.String
		}
		if telefono.Valid {
			t.Telefono = telefono.String
		}

		data.Data = t
		renderTemplate(w, "tecnici_form.html", data)
		return
	}

	// POST - aggiorna il tecnico
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		r.ParseForm()
	}

	nome := strings.TrimSpace(r.FormValue("nome"))
	cognome := strings.TrimSpace(r.FormValue("cognome"))
	email := strings.TrimSpace(r.FormValue("email"))
	telefono := strings.TrimSpace(r.FormValue("telefono"))
	ruolo := models.Ruolo(r.FormValue("ruolo"))
	attivo := r.FormValue("attivo") == "on"
	nuovaPassword := r.FormValue("nuova_password")

	// Campi SMTP
	smtpServer := strings.TrimSpace(r.FormValue("smtp_server"))
	smtpPortStr := r.FormValue("smtp_port")
	smtpPort := 587
	if p, err := strconv.Atoi(smtpPortStr); err == nil && p > 0 {
		smtpPort = p
	}
	smtpUser := strings.TrimSpace(r.FormValue("smtp_user"))
	smtpPassword := r.FormValue("smtp_password")

	// Validazione
	if nome == "" || cognome == "" || email == "" {
		data.Error = "Compila tutti i campi obbligatori"
		renderTemplate(w, "tecnici_form.html", data)
		return
	}

	// Verifica che non si stia disattivando se stesso
	session := middleware.GetSession(r)
	if session != nil && session.UserID == id && !attivo {
		data.Error = "Non puoi disattivare il tuo stesso account"
		renderTemplate(w, "tecnici_form.html", data)
		return
	}

	// Gestione upload nuovo documento
	var updateDocumento bool
	var documentoPath string
	file, header, err := r.FormFile("documento")
	if err == nil {
		defer file.Close()
		updateDocumento = true

		uploadDir := filepath.Join("web", "static", "uploads", "documenti")
		os.MkdirAll(uploadDir, 0755)

		ext := filepath.Ext(header.Filename)
		newFilename := fmt.Sprintf("doc_%d_%d%s", id, time.Now().Unix(), ext)
		filePath := filepath.Join(uploadDir, newFilename)

		dst, err := os.Create(filePath)
		if err == nil {
			defer dst.Close()
			io.Copy(dst, file)
			documentoPath = filepath.Join("uploads", "documenti", newFilename)
		}
	}

	// Aggiorna nel database
	if updateDocumento {
		_, err = database.DB.Exec(`
			UPDATE utenti SET nome = ?, cognome = ?, email = ?, telefono = ?, ruolo = ?, attivo = ?, documento_path = ?, smtp_server = ?, smtp_port = ?, smtp_user = ?, smtp_password = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, nome, cognome, email, telefono, ruolo, attivo, documentoPath, smtpServer, smtpPort, smtpUser, smtpPassword, id)
	} else {
		_, err = database.DB.Exec(`
			UPDATE utenti SET nome = ?, cognome = ?, email = ?, telefono = ?, ruolo = ?, attivo = ?, smtp_server = ?, smtp_port = ?, smtp_user = ?, smtp_password = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, nome, cognome, email, telefono, ruolo, attivo, smtpServer, smtpPort, smtpUser, smtpPassword, id)
	}

	if err != nil {
		data.Error = "Errore durante il salvataggio"
		renderTemplate(w, "tecnici_form.html", data)
		return
	}

	// Aggiorna password se specificata
	if nuovaPassword != "" {
		auth.UpdatePassword(id, nuovaPassword)
	}

	http.Redirect(w, r, "/tecnici?success=modificato", http.StatusSeeOther)
}

// EliminaTecnico gestisce l'eliminazione di un tecnico
func EliminaTecnico(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/tecnici", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/tecnici", http.StatusSeeOther)
		return
	}

	// Verifica che non si stia eliminando se stesso
	session := middleware.GetSession(r)
	if session != nil && session.UserID == id {
		http.Redirect(w, r, "/tecnici?error=no_self_delete", http.StatusSeeOther)
		return
	}

	// Elimina documento se esiste
	var docPath sql.NullString
	database.DB.QueryRow("SELECT documento_path FROM utenti WHERE id = ?", id).Scan(&docPath)
	if docPath.Valid && docPath.String != "" {
		os.Remove(filepath.Join("web", "static", docPath.String))
	}

	_, err = database.DB.Exec("DELETE FROM utenti WHERE id = ?", id)
	if err != nil {
		http.Redirect(w, r, "/tecnici?error=delete_failed", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tecnici?success=eliminato", http.StatusSeeOther)
}
