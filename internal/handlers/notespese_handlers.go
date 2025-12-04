package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"furviogest/internal/auth"
	"furviogest/internal/database"
	"furviogest/internal/middleware"
)

// NotaSpesa struttura nota spesa
type NotaSpesa struct {
	ID              int64
	TecnicoID       int64
	TrasfertaID     *int64
	Data            string
	TipoSpesa       string
	Descrizione     string
	Importo         float64
	MetodoPagamento string
	RicevutaPath    string
	Note            string
	// Campi join
	NomeTecnico string
}

// ListaNoteSpese mostra lista note spese
func ListaNoteSpese(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	tecnicoFilter := r.URL.Query().Get("tecnico")
	meseFilter := r.URL.Query().Get("mese")
	annoFilter := r.URL.Query().Get("anno")

	query := `
		SELECT n.id, n.tecnico_id, n.trasferta_id, n.data, n.tipo_spesa, n.descrizione,
		       n.importo, n.metodo_pagamento, COALESCE(n.ricevuta_path, ''),
		       COALESCE(n.note, ''),
		       COALESCE(u.cognome || ' ' || u.nome, '') as tecnico
		FROM note_spese n
		LEFT JOIN utenti u ON n.tecnico_id = u.id
		WHERE n.deleted_at IS NULL
	`

	var args []interface{}

	// Se non tecnico, mostra solo le proprie
	if !session.IsTecnico() {
		query += " AND n.tecnico_id = ?"
		args = append(args, session.UserID)
	} else if tecnicoFilter != "" {
		query += " AND n.tecnico_id = ?"
		args = append(args, tecnicoFilter)
	}

	if meseFilter != "" && annoFilter != "" {
		query += " AND strftime('%m', n.data) = ? AND strftime('%Y', n.data) = ?"
		args = append(args, meseFilter, annoFilter)
	}

	query += " ORDER BY n.data DESC LIMIT 200"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Errore caricamento note spese: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var noteSpese []NotaSpesa
	for rows.Next() {
		var n NotaSpesa
		var trasfID *int64
		err := rows.Scan(&n.ID, &n.TecnicoID, &trasfID, &n.Data, &n.TipoSpesa, &n.Descrizione,
			&n.Importo, &n.MetodoPagamento, &n.RicevutaPath, &n.Note, &n.NomeTecnico)
		if err != nil {
			continue
		}
		n.TrasfertaID = trasfID
		noteSpese = append(noteSpese, n)
	}

	tecnici, _ := getTecniciList()

	pageData := NewPageData("Note Spese", r)
	pageData.Data = map[string]interface{}{
		"NoteSpese":     noteSpese,
		"Tecnici":       tecnici,
		"TecnicoFilter": tecnicoFilter,
		"MeseFilter":    meseFilter,
		"AnnoFilter":    annoFilter,
	}

	renderTemplate(w, "notespese_lista.html", pageData)
}

// NuovaNotaSpesa gestisce creazione nuova nota spesa
func NuovaNotaSpesa(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == "POST" {
		err := r.ParseMultipartForm(10 << 20) // 10MB
		if err != nil {
			r.ParseForm()
		}

		tecnicoID, _ := strconv.ParseInt(r.FormValue("tecnico_id"), 10, 64)
		data := r.FormValue("data")
		tipoSpesa := r.FormValue("tipo_spesa")
		descrizione := r.FormValue("descrizione")
		importo, _ := strconv.ParseFloat(r.FormValue("importo"), 64)
		metodoPagamento := r.FormValue("metodo_pagamento")
		note := r.FormValue("note")

		if !session.IsTecnico() {
			tecnicoID = session.UserID
		}

		// Upload ricevuta se presente
		var ricevutaPath string
		file, header, err := r.FormFile("ricevuta")
		if err == nil {
			defer file.Close()
			uploadDir := filepath.Join("web", "static", "uploads", "ricevute")
			os.MkdirAll(uploadDir, 0755)
			ext := filepath.Ext(header.Filename)
			fileName := fmt.Sprintf("%d_%d%s", tecnicoID, time.Now().UnixNano(), ext)
			filePath := filepath.Join(uploadDir, fileName)
			dst, err := os.Create(filePath)
			if err == nil {
				defer dst.Close()
				io.Copy(dst, file)
				ricevutaPath = "/static/uploads/ricevute/" + fileName
			}
		}

		_, err = database.DB.Exec(`
			INSERT INTO note_spese (tecnico_id, data, tipo_spesa, descrizione, importo, metodo_pagamento, ricevuta_path, note)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, tecnicoID, data, tipoSpesa, descrizione, importo, metodoPagamento, ricevutaPath, note)

		if err != nil {
			pageData := NewPageData("Nuova Nota Spesa", r)
			pageData.Error = "Errore creazione nota spesa: " + err.Error()
			pageData.Data = getNotaSpesaFormData(0, session)
			renderTemplate(w, "notaspesa_form.html", pageData)
			return
		}

		http.Redirect(w, r, "/note-spese", http.StatusSeeOther)
		return
	}

	pageData := NewPageData("Nuova Nota Spesa", r)
	pageData.Data = getNotaSpesaFormData(0, session)
	renderTemplate(w, "notaspesa_form.html", pageData)
}

// ModificaNotaSpesa gestisce modifica nota spesa
func ModificaNotaSpesa(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/note-spese/modifica/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/note-spese", http.StatusSeeOther)
		return
	}

	if r.Method == "POST" {
		r.ParseMultipartForm(10 << 20)

		tecnicoID, _ := strconv.ParseInt(r.FormValue("tecnico_id"), 10, 64)
		data := r.FormValue("data")
		tipoSpesa := r.FormValue("tipo_spesa")
		descrizione := r.FormValue("descrizione")
		importo, _ := strconv.ParseFloat(r.FormValue("importo"), 64)
		metodoPagamento := r.FormValue("metodo_pagamento")
		note := r.FormValue("note")

		if !session.IsTecnico() {
			tecnicoID = session.UserID
		}

		// Upload nuova ricevuta se presente
		var ricevutaPath string
		file, header, err := r.FormFile("ricevuta")
		if err == nil {
			defer file.Close()
			uploadDir := filepath.Join("web", "static", "uploads", "ricevute")
			os.MkdirAll(uploadDir, 0755)
			ext := filepath.Ext(header.Filename)
			fileName := fmt.Sprintf("%d_%d%s", tecnicoID, time.Now().UnixNano(), ext)
			filePath := filepath.Join(uploadDir, fileName)
			dst, err := os.Create(filePath)
			if err == nil {
				defer dst.Close()
				io.Copy(dst, file)
				ricevutaPath = "/static/uploads/ricevute/" + fileName
			}
		}

		if ricevutaPath != "" {
			_, err = database.DB.Exec(`
				UPDATE note_spese 
				SET tecnico_id = ?, data = ?, tipo_spesa = ?, descrizione = ?, 
				    importo = ?, metodo_pagamento = ?, ricevuta_path = ?, note = ?,
				    updated_at = CURRENT_TIMESTAMP
				WHERE id = ?
			`, tecnicoID, data, tipoSpesa, descrizione, importo, metodoPagamento, ricevutaPath, note, id)
		} else {
			_, err = database.DB.Exec(`
				UPDATE note_spese 
				SET tecnico_id = ?, data = ?, tipo_spesa = ?, descrizione = ?, 
				    importo = ?, metodo_pagamento = ?, note = ?,
				    updated_at = CURRENT_TIMESTAMP
				WHERE id = ?
			`, tecnicoID, data, tipoSpesa, descrizione, importo, metodoPagamento, note, id)
		}

		if err != nil {
			pageData := NewPageData("Modifica Nota Spesa", r)
			pageData.Error = "Errore modifica nota spesa: " + err.Error()
			pageData.Data = getNotaSpesaFormData(id, session)
			renderTemplate(w, "notaspesa_form.html", pageData)
			return
		}

		http.Redirect(w, r, "/note-spese", http.StatusSeeOther)
		return
	}

	pageData := NewPageData("Modifica Nota Spesa", r)
	pageData.Data = getNotaSpesaFormData(id, session)
	renderTemplate(w, "notaspesa_form.html", pageData)
}

// EliminaNotaSpesa soft delete nota spesa
func EliminaNotaSpesa(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/note-spese/elimina/")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	database.DB.Exec("UPDATE note_spese SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", id)

	http.Redirect(w, r, "/note-spese", http.StatusSeeOther)
}

func getNotaSpesaFormData(notaID int64, session *auth.Session) map[string]interface{} {
	data := make(map[string]interface{})

	if session.IsTecnico() {
		tecnici, _ := getTecniciList()
		data["Tecnici"] = tecnici
	}

	// Tipi spesa
	data["TipiSpesa"] = []map[string]string{
		{"Value": "carburante", "Label": "Carburante"},
		{"Value": "hotel", "Label": "Hotel/Alloggio"},
		{"Value": "pranzo", "Label": "Pranzo"},
		{"Value": "cena", "Label": "Cena"},
		{"Value": "materiali", "Label": "Materiali"},
		{"Value": "varie", "Label": "Altre Spese"},
	}

	if notaID > 0 {
		var n NotaSpesa
		var trasfID *int64
		database.DB.QueryRow(`
			SELECT id, tecnico_id, trasferta_id, data, tipo_spesa, descrizione,
			       importo, metodo_pagamento, COALESCE(ricevuta_path, ''), COALESCE(note, '')
			FROM note_spese WHERE id = ?
		`, notaID).Scan(&n.ID, &n.TecnicoID, &trasfID, &n.Data, &n.TipoSpesa, &n.Descrizione,
			&n.Importo, &n.MetodoPagamento, &n.RicevutaPath, &n.Note)
		n.TrasfertaID = trasfID
		data["NotaSpesa"] = n
	} else {
		data["NotaSpesa"] = NotaSpesa{
			TecnicoID:       session.UserID,
			Data:            time.Now().Format("2006-01-02"),
			MetodoPagamento: "tecnico",
		}
	}

	return data
}
