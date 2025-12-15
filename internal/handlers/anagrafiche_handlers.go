package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"furviogest/internal/database"
	"furviogest/internal/models"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ============================================
// FORNITORI
// ============================================

func ListaFornitori(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Fornitori - FurvioGest", r)

	rows, err := database.DB.Query(`
		SELECT id, nome, partita_iva, codice_fiscale, indirizzo, cap, citta, provincia, nazione, telefono, cellulare, email, referente, telefono_referente, note, created_at, COALESCE(is_amazon, 0) as is_amazon
		FROM fornitori ORDER BY nome
	`)
	if err != nil {
		data.Error = "Errore nel recupero dei fornitori"
		renderTemplate(w, "fornitori_lista.html", data)
		return
	}
	defer rows.Close()

	var fornitori []models.Fornitore
	for rows.Next() {
		var f models.Fornitore
		var partitaIVA, codiceFiscale, indirizzo, cap, citta, provincia, nazione, telefono, cellulare, email, referente, telefonoReferente, note sql.NullString
		err := rows.Scan(&f.ID, &f.Nome, &partitaIVA, &codiceFiscale, &indirizzo, &cap, &citta, &provincia, &nazione, &telefono, &cellulare, &email, &referente, &telefonoReferente, &note, &f.CreatedAt, &f.IsAmazon)
		if err != nil {
			continue
		}
		if partitaIVA.Valid {
			f.PartitaIVA = partitaIVA.String
		}
		if codiceFiscale.Valid {
			f.CodiceFiscale = codiceFiscale.String
		}
		if indirizzo.Valid {
			f.Indirizzo = indirizzo.String
		}
		if cap.Valid {
			f.CAP = cap.String
		}
		if citta.Valid {
			f.Citta = citta.String
		}
		if provincia.Valid {
			f.Provincia = provincia.String
		}
		if nazione.Valid {
			f.Nazione = nazione.String
		}
		if telefono.Valid {
			f.Telefono = telefono.String
		}
		if cellulare.Valid {
			f.Cellulare = cellulare.String
		}
		if email.Valid {
			f.Email = email.String
		}
		if referente.Valid {
			f.Referente = referente.String
		}
		if telefonoReferente.Valid {
			f.TelefonoReferente = telefonoReferente.String
		}
		if note.Valid {
			f.Note = note.String
		}
		fornitori = append(fornitori, f)
	}

	data.Data = fornitori
	renderTemplate(w, "fornitori_lista.html", data)
}

func NuovoFornitore(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Nuovo Fornitore - FurvioGest", r)

	if r.Method == http.MethodGet {
		renderTemplate(w, "fornitori_form.html", data)
		return
	}

	r.ParseMultipartForm(10 << 20)
	nome := strings.TrimSpace(r.FormValue("nome"))
	partitaIVA := strings.TrimSpace(r.FormValue("partita_iva"))
	codiceFiscale := strings.TrimSpace(r.FormValue("codice_fiscale"))
	indirizzo := strings.TrimSpace(r.FormValue("indirizzo"))
	cap := strings.TrimSpace(r.FormValue("cap"))
	citta := strings.TrimSpace(r.FormValue("citta"))
	provincia := strings.ToUpper(strings.TrimSpace(r.FormValue("provincia")))
	nazione := strings.TrimSpace(r.FormValue("nazione"))
	if nazione == "" {
		nazione = "Italia"
	}
	telefono := strings.TrimSpace(r.FormValue("telefono"))
	cellulare := strings.TrimSpace(r.FormValue("cellulare"))
	email := strings.TrimSpace(r.FormValue("email"))
	referente := strings.TrimSpace(r.FormValue("referente"))
	telefonoReferente := strings.TrimSpace(r.FormValue("telefono_referente"))
	note := strings.TrimSpace(r.FormValue("note"))

	if nome == "" {
		data.Error = "La ragione sociale è obbligatoria"
		renderTemplate(w, "fornitori_form.html", data)
		return
	}

	_, err := database.DB.Exec(`
		INSERT INTO fornitori (nome, partita_iva, codice_fiscale, indirizzo, cap, citta, provincia, nazione, telefono, cellulare, email, referente, telefono_referente, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, nome, partitaIVA, codiceFiscale, indirizzo, cap, citta, provincia, nazione, telefono, cellulare, email, referente, telefonoReferente, note)

	if err != nil {
		data.Error = "Errore durante il salvataggio: " + err.Error()
		renderTemplate(w, "fornitori_form.html", data)
		return
	}

	http.Redirect(w, r, "/fornitori", http.StatusSeeOther)
}

func ModificaFornitore(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Modifica Fornitore - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/fornitori", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/fornitori", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		var f models.Fornitore
		var partitaIVA, codiceFiscale, indirizzo, cap, citta, provincia, nazione, telefono, cellulare, email, referente, telefonoReferente, note sql.NullString
		err := database.DB.QueryRow(`
			SELECT id, nome, partita_iva, codice_fiscale, indirizzo, cap, citta, provincia, nazione, telefono, cellulare, email, referente, telefono_referente, note
			FROM fornitori WHERE id = ?
		`, id).Scan(&f.ID, &f.Nome, &partitaIVA, &codiceFiscale, &indirizzo, &cap, &citta, &provincia, &nazione, &telefono, &cellulare, &email, &referente, &telefonoReferente, &note)

		if err != nil {
			http.Redirect(w, r, "/fornitori", http.StatusSeeOther)
			return
		}
		if partitaIVA.Valid {
			f.PartitaIVA = partitaIVA.String
		}
		if codiceFiscale.Valid {
			f.CodiceFiscale = codiceFiscale.String
		}
		if indirizzo.Valid {
			f.Indirizzo = indirizzo.String
		}
		if cap.Valid {
			f.CAP = cap.String
		}
		if citta.Valid {
			f.Citta = citta.String
		}
		if provincia.Valid {
			f.Provincia = provincia.String
		}
		if nazione.Valid {
			f.Nazione = nazione.String
		} else {
			f.Nazione = "Italia"
		}
		if telefono.Valid {
			f.Telefono = telefono.String
		}
		if cellulare.Valid {
			f.Cellulare = cellulare.String
		}
		if email.Valid {
			f.Email = email.String
		}
		if referente.Valid {
			f.Referente = referente.String
		}
		if telefonoReferente.Valid {
			f.TelefonoReferente = telefonoReferente.String
		}
		if note.Valid {
			f.Note = note.String
		}

		data.Data = f
		renderTemplate(w, "fornitori_form.html", data)
		return
	}

	r.ParseMultipartForm(10 << 20)
	nome := strings.TrimSpace(r.FormValue("nome"))
	partitaIVA := strings.TrimSpace(r.FormValue("partita_iva"))
	codiceFiscale := strings.TrimSpace(r.FormValue("codice_fiscale"))
	indirizzo := strings.TrimSpace(r.FormValue("indirizzo"))
	cap := strings.TrimSpace(r.FormValue("cap"))
	citta := strings.TrimSpace(r.FormValue("citta"))
	provincia := strings.ToUpper(strings.TrimSpace(r.FormValue("provincia")))
	nazione := strings.TrimSpace(r.FormValue("nazione"))
	if nazione == "" {
		nazione = "Italia"
	}
	telefono := strings.TrimSpace(r.FormValue("telefono"))
	cellulare := strings.TrimSpace(r.FormValue("cellulare"))
	email := strings.TrimSpace(r.FormValue("email"))
	referente := strings.TrimSpace(r.FormValue("referente"))
	telefonoReferente := strings.TrimSpace(r.FormValue("telefono_referente"))
	note := strings.TrimSpace(r.FormValue("note"))

	if nome == "" {
		data.Error = "La ragione sociale è obbligatoria"
		renderTemplate(w, "fornitori_form.html", data)
		return
	}

	_, err = database.DB.Exec(`
		UPDATE fornitori SET nome = ?, partita_iva = ?, codice_fiscale = ?, indirizzo = ?, cap = ?, citta = ?, provincia = ?, nazione = ?, telefono = ?, cellulare = ?, email = ?, referente = ?, telefono_referente = ?, note = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, nome, partitaIVA, codiceFiscale, indirizzo, cap, citta, provincia, nazione, telefono, cellulare, email, referente, telefonoReferente, note, id)

	if err != nil {
		data.Error = "Errore durante il salvataggio: " + err.Error()
		renderTemplate(w, "fornitori_form.html", data)
		return
	}

	http.Redirect(w, r, "/fornitori", http.StatusSeeOther)
}

func EliminaFornitore(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/fornitori", http.StatusSeeOther)
		return
	}

	id, _ := strconv.ParseInt(pathParts[3], 10, 64)
	
	// Non permettere eliminazione di Amazon
	var isAmazon bool
	database.DB.QueryRow(`SELECT COALESCE(is_amazon, 0) FROM fornitori WHERE id = ?`, id).Scan(&isAmazon)
	if isAmazon {
		http.Redirect(w, r, "/fornitori", http.StatusSeeOther)
		return
	}
	
	tx, err := database.DB.Begin()
	if err != nil {
		http.Redirect(w, r, "/fornitori", http.StatusSeeOther)
		return
	}

	// 1. Trova tutti i DDT di questo fornitore
	ddtRows, _ := tx.Query(`SELECT id FROM ddt_fatture WHERE fornitore_id = ?`, id)
	var ddtIDs []int64
	for ddtRows.Next() {
		var ddtID int64
		ddtRows.Scan(&ddtID)
		ddtIDs = append(ddtIDs, ddtID)
	}
	ddtRows.Close()

	// 2. Per ogni DDT, trova prodotti collegati e ricalcola giacenze
	var prodottiDaVerificare []int64
	for _, ddtID := range ddtIDs {
		movRows, _ := tx.Query(`SELECT prodotto_id, quantita FROM movimenti_acquisto WHERE ddt_fattura_id = ?`, ddtID)
		
		for movRows.Next() {
			var prodID int64
			var qta int
			movRows.Scan(&prodID, &qta)
			
			// Sottrai giacenza
			tx.Exec(`UPDATE prodotti SET giacenza = giacenza - ? WHERE id = ?`, qta, prodID)
			prodottiDaVerificare = append(prodottiDaVerificare, prodID)
		}
		movRows.Close()
		
		// Elimina movimenti_acquisto del DDT
		tx.Exec(`DELETE FROM movimenti_acquisto WHERE ddt_fattura_id = ?`, ddtID)
	}

	// 3. Elimina DDT del fornitore
	tx.Exec(`DELETE FROM ddt_fatture WHERE fornitore_id = ?`, id)

	// 4. Elimina archivio PDF del fornitore
	pdfRows, _ := tx.Query(`SELECT file_path FROM archivio_pdf WHERE fornitore_id = ?`, id)
	for pdfRows.Next() {
		var filePath string
		pdfRows.Scan(&filePath)
		if filePath != "" {
			os.Remove("/home/ies/furviogest/uploads/archivio_pdf/" + filePath)
		}
	}
	pdfRows.Close()
	tx.Exec(`DELETE FROM archivio_pdf WHERE fornitore_id = ?`, id)

	// 5. Verifica ed elimina prodotti senza più movimenti
	for _, prodID := range prodottiDaVerificare {
		var countMov int
		tx.QueryRow(`SELECT COUNT(*) FROM movimenti_acquisto WHERE prodotto_id = ?`, prodID).Scan(&countMov)
		if countMov == 0 {
			// Nessun altro movimento, elimina prodotto non-spare
			tx.Exec(`DELETE FROM prodotti WHERE id = ? AND origine = 'nuovo'`, prodID)
		}
	}

	// 6. Elimina il fornitore
	tx.Exec(`DELETE FROM fornitori WHERE id = ?`, id)

	tx.Commit()
	http.Redirect(w, r, "/fornitori", http.StatusSeeOther)
}

// APIInfoEliminazioneFornitore restituisce info su cosa verrà eliminato
func APIInfoEliminazioneFornitore(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	var info struct {
		NomeFornitore     string   `json:"nome_fornitore"`
		NumDDT            int      `json:"num_ddt"`
		IsAmazon          bool     `json:"is_amazon"`
		ProdottiEliminati []string `json:"prodotti_eliminati"`
	}

	database.DB.QueryRow(`SELECT nome, COALESCE(is_amazon, 0) FROM fornitori WHERE id = ?`, id).Scan(&info.NomeFornitore, &info.IsAmazon)
	database.DB.QueryRow(`SELECT COUNT(*) FROM ddt_fatture WHERE fornitore_id = ?`, id).Scan(&info.NumDDT)

	// Prodotti che verranno eliminati
	rows, _ := database.DB.Query(`
		SELECT DISTINCT p.nome FROM prodotti p
		JOIN movimenti_acquisto m ON m.prodotto_id = p.id
		JOIN ddt_fatture d ON m.ddt_fattura_id = d.id
		WHERE d.fornitore_id = ? AND p.origine = 'nuovo'
		AND NOT EXISTS (
			SELECT 1 FROM movimenti_acquisto m2
			JOIN ddt_fatture d2 ON m2.ddt_fattura_id = d2.id
			WHERE m2.prodotto_id = p.id AND d2.fornitore_id != ?
		)
	`, id, id)
	defer rows.Close()
	for rows.Next() {
		var nome string
		rows.Scan(&nome)
		info.ProdottiEliminati = append(info.ProdottiEliminati, nome)
	}

	json.NewEncoder(w).Encode(info)
}

// ============================================
// PORTI
// ============================================

func ListaPorti(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Porti - FurvioGest", r)

	rows, err := database.DB.Query(`
		SELECT id, nome, citta, paese, nome_agenzia, email_agenzia, telefono_agenzia, note, created_at
		FROM porti ORDER BY nome
	`)
	if err != nil {
		data.Error = "Errore nel recupero dei porti"
		renderTemplate(w, "porti_lista.html", data)
		return
	}
	defer rows.Close()

	var porti []models.Porto
	for rows.Next() {
		var p models.Porto
		var citta, paese, nomeAgenzia, emailAgenzia, telefonoAgenzia, note sql.NullString
		err := rows.Scan(&p.ID, &p.Nome, &citta, &paese, &nomeAgenzia, &emailAgenzia, &telefonoAgenzia, &note, &p.CreatedAt)
		if err != nil {
			continue
		}
		if citta.Valid {
			p.Citta = citta.String
		}
		if paese.Valid {
			p.Paese = paese.String
		}
		if nomeAgenzia.Valid {
			p.NomeAgenzia = nomeAgenzia.String
		}
		if emailAgenzia.Valid {
			p.EmailAgenzia = emailAgenzia.String
		}
		if telefonoAgenzia.Valid {
			p.TelefonoAgenzia = telefonoAgenzia.String
		}
		if note.Valid {
			p.Note = note.String
		}
		porti = append(porti, p)
	}

	data.Data = porti
	renderTemplate(w, "porti_lista.html", data)
}

func NuovoPorto(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Nuovo Porto - FurvioGest", r)

	if r.Method == http.MethodGet {
		renderTemplate(w, "porti_form.html", data)
		return
	}

	r.ParseMultipartForm(10 << 20)
	nome := strings.TrimSpace(r.FormValue("nome"))
	citta := strings.TrimSpace(r.FormValue("citta"))
	paese := strings.TrimSpace(r.FormValue("paese"))
	nomeAgenzia := strings.TrimSpace(r.FormValue("nome_agenzia"))
	emailAgenzia := strings.TrimSpace(r.FormValue("email_agenzia"))
	telefonoAgenzia := strings.TrimSpace(r.FormValue("telefono_agenzia"))
	note := strings.TrimSpace(r.FormValue("note"))
	emailDestinatari := r.FormValue("email_destinatari")
	if emailDestinatari == "" {
		emailDestinatari = "solo_agenzia"
	}

	if nome == "" {
		data.Error = "Il nome è obbligatorio"
		renderTemplate(w, "porti_form.html", data)
		return
	}

	_, err := database.DB.Exec(`
		INSERT INTO porti (nome, citta, paese, nome_agenzia, email_agenzia, telefono_agenzia, note)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, nome, citta, paese, nomeAgenzia, emailAgenzia, telefonoAgenzia, note)

	if err != nil {
		data.Error = "Errore durante il salvataggio"
		renderTemplate(w, "porti_form.html", data)
		return
	}

	http.Redirect(w, r, "/porti", http.StatusSeeOther)
}

func ModificaPorto(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Modifica Porto - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/porti", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/porti", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		var p models.Porto
		var citta, paese, nomeAgenzia, emailAgenzia, telefonoAgenzia, note sql.NullString
		err := database.DB.QueryRow(`
			SELECT id, nome, citta, paese, nome_agenzia, email_agenzia, telefono_agenzia, note
			FROM porti WHERE id = ?
		`, id).Scan(&p.ID, &p.Nome, &citta, &paese, &nomeAgenzia, &emailAgenzia, &telefonoAgenzia, &note)

		if err != nil {
			http.Redirect(w, r, "/porti", http.StatusSeeOther)
			return
		}
		if citta.Valid {
			p.Citta = citta.String
		}
		if paese.Valid {
			p.Paese = paese.String
		}
		if nomeAgenzia.Valid {
			p.NomeAgenzia = nomeAgenzia.String
		}
		if emailAgenzia.Valid {
			p.EmailAgenzia = emailAgenzia.String
		}
		if telefonoAgenzia.Valid {
			p.TelefonoAgenzia = telefonoAgenzia.String
		}
		if note.Valid {
			p.Note = note.String
		}

		data.Data = p
		renderTemplate(w, "porti_form.html", data)
		return
	}

	r.ParseMultipartForm(10 << 20)
	nome := strings.TrimSpace(r.FormValue("nome"))
	citta := strings.TrimSpace(r.FormValue("citta"))
	paese := strings.TrimSpace(r.FormValue("paese"))
	nomeAgenzia := strings.TrimSpace(r.FormValue("nome_agenzia"))
	emailAgenzia := strings.TrimSpace(r.FormValue("email_agenzia"))
	telefonoAgenzia := strings.TrimSpace(r.FormValue("telefono_agenzia"))
	note := strings.TrimSpace(r.FormValue("note"))
	emailDestinatari := r.FormValue("email_destinatari")
	if emailDestinatari == "" {
		emailDestinatari = "solo_agenzia"
	}

	if nome == "" {
		data.Error = "Il nome è obbligatorio"
		renderTemplate(w, "porti_form.html", data)
		return
	}

	_, err = database.DB.Exec(`
		UPDATE porti SET nome = ?, citta = ?, paese = ?, nome_agenzia = ?, email_agenzia = ?, telefono_agenzia = ?, note = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, nome, citta, paese, nomeAgenzia, emailAgenzia, telefonoAgenzia, note, id)

	if err != nil {
		data.Error = "Errore durante il salvataggio"
		renderTemplate(w, "porti_form.html", data)
		return
	}

	http.Redirect(w, r, "/porti", http.StatusSeeOther)
}

func EliminaPorto(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/porti", http.StatusSeeOther)
		return
	}

	id, _ := strconv.ParseInt(pathParts[3], 10, 64)
	database.DB.Exec("DELETE FROM porti WHERE id = ?", id)
	http.Redirect(w, r, "/porti", http.StatusSeeOther)
}

// ============================================
// AUTOMEZZI
// ============================================

func ListaAutomezzi(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Automezzi - FurvioGest", r)

	rows, err := database.DB.Query(`
		SELECT id, targa, marca, modello, libretto_path, note, created_at
		FROM automezzi ORDER BY targa
	`)
	if err != nil {
		data.Error = "Errore nel recupero degli automezzi"
		renderTemplate(w, "automezzi_lista.html", data)
		return
	}
	defer rows.Close()

	var automezzi []models.Automezzo
	for rows.Next() {
		var a models.Automezzo
		var marca, modello, libretto, note sql.NullString
		err := rows.Scan(&a.ID, &a.Targa, &marca, &modello, &libretto, &note, &a.CreatedAt)
		if err != nil {
			continue
		}
		if marca.Valid {
			a.Marca = marca.String
		}
		if modello.Valid {
			a.Modello = modello.String
		}
		if libretto.Valid {
			a.LibrettoPath = libretto.String
		}
		if note.Valid {
			a.Note = note.String
		}
		automezzi = append(automezzi, a)
	}

	data.Data = automezzi
	renderTemplate(w, "automezzi_lista.html", data)
}

func NuovoAutomezzo(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Nuovo Automezzo - FurvioGest", r)

	if r.Method == http.MethodGet {
		renderTemplate(w, "automezzi_form.html", data)
		return
	}

	// Parse del form con gestione errore
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		// Fallback: prova con ParseForm standard
		r.ParseForm()
	}

	targa := strings.TrimSpace(r.FormValue("targa"))
	marca := strings.TrimSpace(r.FormValue("marca"))
	modello := strings.TrimSpace(r.FormValue("modello"))
	note := strings.TrimSpace(r.FormValue("note"))
	emailDestinatari := r.FormValue("email_destinatari")
	if emailDestinatari == "" {
		emailDestinatari = "solo_agenzia"
	}

	// Conserva i valori inseriti in caso di errore
	if targa == "" {
		data.Error = "La targa è obbligatoria"
		data.Data = models.Automezzo{Targa: targa, Marca: marca, Modello: modello, Note: note}
		renderTemplate(w, "automezzi_form.html", data)
		return
	}

	// Gestione upload libretto
	var librettoPath string
	file, header, err := r.FormFile("libretto")
	if err == nil {
		defer file.Close()
		// Crea nome file univoco
		ext := filepath.Ext(header.Filename)
		newFileName := fmt.Sprintf("libretto_%s_%d%s", targa, time.Now().Unix(), ext)
		uploadDir := filepath.Join("web", "static", "uploads", "libretti")
		os.MkdirAll(uploadDir, 0755)
		uploadPath := filepath.Join(uploadDir, newFileName)
		
		dst, err := os.Create(uploadPath)
		if err == nil {
			defer dst.Close()
			io.Copy(dst, file)
			librettoPath = "uploads/libretti/" + newFileName
		}
	}

	_, err = database.DB.Exec(`
		INSERT INTO automezzi (targa, marca, modello, note, libretto_path)
		VALUES (?, ?, ?, ?, ?)
	`, targa, marca, modello, note, librettoPath)

	if err != nil {
		data.Error = "Errore durante il salvataggio (targa già esistente?)"
		renderTemplate(w, "automezzi_form.html", data)
		return
	}

	http.Redirect(w, r, "/automezzi", http.StatusSeeOther)
}

func ModificaAutomezzo(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Modifica Automezzo - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/automezzi", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/automezzi", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		var a models.Automezzo
		var marca, modello, libretto, note sql.NullString
		err := database.DB.QueryRow(`
			SELECT id, targa, marca, modello, libretto_path, note
			FROM automezzi WHERE id = ?
		`, id).Scan(&a.ID, &a.Targa, &marca, &modello, &libretto, &note)

		if err != nil {
			http.Redirect(w, r, "/automezzi", http.StatusSeeOther)
			return
		}
		if marca.Valid {
			a.Marca = marca.String
		}
		if modello.Valid {
			a.Modello = modello.String
		}
		if libretto.Valid {
			a.LibrettoPath = libretto.String
		}
		if note.Valid {
			a.Note = note.String
		}

		data.Data = a
		renderTemplate(w, "automezzi_form.html", data)
		return
	}

	// Parse del form con gestione errore
	parseErr := r.ParseMultipartForm(10 << 20)
	if parseErr != nil {
		r.ParseForm()
	}

	targa := strings.TrimSpace(r.FormValue("targa"))
	marca := strings.TrimSpace(r.FormValue("marca"))
	modello := strings.TrimSpace(r.FormValue("modello"))
	note := strings.TrimSpace(r.FormValue("note"))
	emailDestinatari := r.FormValue("email_destinatari")
	if emailDestinatari == "" {
		emailDestinatari = "solo_agenzia"
	}

	if targa == "" {
		data.Error = "La targa è obbligatoria"
		data.Data = models.Automezzo{ID: id, Targa: targa, Marca: marca, Modello: modello, Note: note}
		renderTemplate(w, "automezzi_form.html", data)
		return
	}

	// Gestione upload libretto
	file, header, fileErr := r.FormFile("libretto")
	if fileErr == nil {
		defer file.Close()
		// Crea nome file univoco
		ext := filepath.Ext(header.Filename)
		newFileName := fmt.Sprintf("libretto_%s_%d%s", targa, time.Now().Unix(), ext)
		uploadDir := filepath.Join("web", "static", "uploads", "libretti")
		os.MkdirAll(uploadDir, 0755)
		uploadPath := filepath.Join(uploadDir, newFileName)
		
		dst, createErr := os.Create(uploadPath)
		if createErr == nil {
			defer dst.Close()
			io.Copy(dst, file)
			librettoPath := "uploads/libretti/" + newFileName
			// Aggiorna con nuovo libretto
			_, err = database.DB.Exec(`
				UPDATE automezzi SET targa = ?, marca = ?, modello = ?, note = ?, libretto_path = ?, updated_at = CURRENT_TIMESTAMP
				WHERE id = ?
			`, targa, marca, modello, note, librettoPath, id)
			if err != nil {
				data.Error = "Errore durante il salvataggio"
				renderTemplate(w, "automezzi_form.html", data)
				return
			}
			http.Redirect(w, r, "/automezzi", http.StatusSeeOther)
			return
		}
	}

	// Aggiorna senza modificare libretto
	_, err = database.DB.Exec(`
		UPDATE automezzi SET targa = ?, marca = ?, modello = ?, note = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, targa, marca, modello, note, id)

	if err != nil {
		data.Error = "Errore durante il salvataggio"
		renderTemplate(w, "automezzi_form.html", data)
		return
	}

	http.Redirect(w, r, "/automezzi", http.StatusSeeOther)
}

func EliminaAutomezzo(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/automezzi", http.StatusSeeOther)
		return
	}

	id, _ := strconv.ParseInt(pathParts[3], 10, 64)
	database.DB.Exec("DELETE FROM automezzi WHERE id = ?", id)
	http.Redirect(w, r, "/automezzi", http.StatusSeeOther)
}

// ============================================
// COMPAGNIE
// ============================================

func ListaCompagnie(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Compagnie - FurvioGest", r)

	rows, err := database.DB.Query(`
		SELECT id, nome, indirizzo, telefono, email, note, created_at
		FROM compagnie ORDER BY nome
	`)
	if err != nil {
		data.Error = "Errore nel recupero delle compagnie"
		renderTemplate(w, "compagnie_lista.html", data)
		return
	}
	defer rows.Close()

	var compagnie []models.Compagnia
	for rows.Next() {
		var c models.Compagnia
		var indirizzo, telefono, email, note sql.NullString
		err := rows.Scan(&c.ID, &c.Nome, &indirizzo, &telefono, &email, &note, &c.CreatedAt)
		if err != nil {
			continue
		}
		if indirizzo.Valid {
			c.Indirizzo = indirizzo.String
		}
		if telefono.Valid {
			c.Telefono = telefono.String
		}
		if email.Valid {
			c.Email = email.String
		}
		if note.Valid {
			c.Note = note.String
		}
		compagnie = append(compagnie, c)
	}

	data.Data = compagnie
	renderTemplate(w, "compagnie_lista.html", data)
}

func NuovaCompagnia(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Nuova Compagnia - FurvioGest", r)

	if r.Method == http.MethodGet {
		renderTemplate(w, "compagnie_form.html", data)
		return
	}

	r.ParseMultipartForm(10 << 20)
	nome := strings.TrimSpace(r.FormValue("nome"))
	indirizzo := strings.TrimSpace(r.FormValue("indirizzo"))
	citta := strings.TrimSpace(r.FormValue("citta"))
	cap := strings.TrimSpace(r.FormValue("cap"))
	provincia := strings.TrimSpace(r.FormValue("provincia"))
	piva := strings.TrimSpace(r.FormValue("piva"))
	codiceFiscale := strings.TrimSpace(r.FormValue("codice_fiscale"))
	telefono := strings.TrimSpace(r.FormValue("telefono"))
	emailVal := strings.TrimSpace(r.FormValue("email"))
	note := strings.TrimSpace(r.FormValue("note"))
	emailDestinatari := r.FormValue("email_destinatari")
	if emailDestinatari == "" {
		emailDestinatari = "solo_agenzia"
	}

	if nome == "" {
		data.Error = "Il nome è obbligatorio"
		renderTemplate(w, "compagnie_form.html", data)
		return
	}

	// Inserisci compagnia
	result, err := database.DB.Exec(`
		INSERT INTO compagnie (nome, indirizzo, citta, cap, provincia, piva, codice_fiscale, telefono, email, note, email_destinatari)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, nome, indirizzo, citta, cap, provincia, piva, codiceFiscale, telefono, emailVal, note, emailDestinatari)

	if err != nil {
		data.Error = "Errore durante il salvataggio"
		renderTemplate(w, "compagnie_form.html", data)
		return
	}

	// Gestisci upload logo
	file, header, err := r.FormFile("logo")
	if err == nil && header != nil {
		defer file.Close()
		compagniaID, _ := result.LastInsertId()
		logoPath := saveCompagniaLogo(compagniaID, file, header)
		if logoPath != "" {
			database.DB.Exec("UPDATE compagnie SET logo = ? WHERE id = ?", logoPath, compagniaID)
		}
	}

	http.Redirect(w, r, "/compagnie", http.StatusSeeOther)
}

func ModificaCompagnia(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Modifica Compagnia - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/compagnie", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/compagnie", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		var c models.Compagnia
		var indirizzo, citta, cap, provincia, piva, cf, telefono, emailVal, note, emailDest, logo sql.NullString
		err := database.DB.QueryRow(`
			SELECT id, nome, indirizzo, citta, cap, provincia, piva, codice_fiscale, 
			       telefono, email, note, COALESCE(email_destinatari, 'solo_agenzia'), logo
			FROM compagnie WHERE id = ?
		`, id).Scan(&c.ID, &c.Nome, &indirizzo, &citta, &cap, &provincia, &piva, &cf,
			&telefono, &emailVal, &note, &emailDest, &logo)

		if err != nil {
			http.Redirect(w, r, "/compagnie", http.StatusSeeOther)
			return
		}
		if indirizzo.Valid { c.Indirizzo = indirizzo.String }
		if citta.Valid { c.Citta = citta.String }
		if cap.Valid { c.CAP = cap.String }
		if provincia.Valid { c.Provincia = provincia.String }
		if piva.Valid { c.PIVA = piva.String }
		if cf.Valid { c.CodiceFiscale = cf.String }
		if telefono.Valid { c.Telefono = telefono.String }
		if emailVal.Valid { c.Email = emailVal.String }
		if note.Valid { c.Note = note.String }
		if emailDest.Valid { c.EmailDestinatari = emailDest.String } else { c.EmailDestinatari = "solo_agenzia" }
		if logo.Valid { c.Logo = logo.String }

		data.Data = c
		renderTemplate(w, "compagnie_form.html", data)
		return
	}

	r.ParseMultipartForm(10 << 20)
	nome := strings.TrimSpace(r.FormValue("nome"))
	indirizzo := strings.TrimSpace(r.FormValue("indirizzo"))
	citta := strings.TrimSpace(r.FormValue("citta"))
	cap := strings.TrimSpace(r.FormValue("cap"))
	provincia := strings.TrimSpace(r.FormValue("provincia"))
	piva := strings.TrimSpace(r.FormValue("piva"))
	codiceFiscale := strings.TrimSpace(r.FormValue("codice_fiscale"))
	telefono := strings.TrimSpace(r.FormValue("telefono"))
	emailVal := strings.TrimSpace(r.FormValue("email"))
	note := strings.TrimSpace(r.FormValue("note"))
	emailDestinatari := r.FormValue("email_destinatari")
	if emailDestinatari == "" {
		emailDestinatari = "solo_agenzia"
	}

	if nome == "" {
		data.Error = "Il nome è obbligatorio"
		renderTemplate(w, "compagnie_form.html", data)
		return
	}

	_, err = database.DB.Exec(`
		UPDATE compagnie SET nome = ?, indirizzo = ?, citta = ?, cap = ?, provincia = ?, 
		       piva = ?, codice_fiscale = ?, telefono = ?, email = ?, note = ?, 
		       email_destinatari = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, nome, indirizzo, citta, cap, provincia, piva, codiceFiscale, telefono, emailVal, note, emailDestinatari, id)

	if err != nil {
		data.Error = "Errore durante il salvataggio"
		renderTemplate(w, "compagnie_form.html", data)
		return
	}

	// Gestisci upload logo
	file, header, err := r.FormFile("logo")
	if err == nil && header != nil {
		defer file.Close()
		logoPath := saveCompagniaLogo(id, file, header)
		if logoPath != "" {
			database.DB.Exec("UPDATE compagnie SET logo = ? WHERE id = ?", logoPath, id)
		}
	}

	http.Redirect(w, r, "/compagnie", http.StatusSeeOther)
}

// saveCompagniaLogo salva il logo della compagnia
func saveCompagniaLogo(compagniaID int64, file multipart.File, header *multipart.FileHeader) string {
	// Crea directory se non esiste
	uploadDir := filepath.Join("uploads", "compagnie")
	os.MkdirAll(uploadDir, 0755)

	// Estensione file
	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("logo_%d%s", compagniaID, ext)
	filePath := filepath.Join(uploadDir, filename)

	// Crea file destinazione
	dst, err := os.Create(filePath)
	if err != nil {
		return ""
	}
	defer dst.Close()

	// Copia contenuto
	_, err = io.Copy(dst, file)
	if err != nil {
		return ""
	}

	return filePath
}

// ServeCompagniaLogo serve il logo di una compagnia
func ServeCompagniaLogo(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var logoPath sql.NullString
	database.DB.QueryRow("SELECT logo FROM compagnie WHERE id = ?", id).Scan(&logoPath)

	if !logoPath.Valid || logoPath.String == "" {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, logoPath.String)
}
func EliminaCompagnia(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/compagnie", http.StatusSeeOther)
		return
	}

	id, _ := strconv.ParseInt(pathParts[3], 10, 64)
	database.DB.Exec("DELETE FROM compagnie WHERE id = ?", id)
	http.Redirect(w, r, "/compagnie", http.StatusSeeOther)
}

// ============================================
// NAVI
// ============================================

type NaveFormData struct {
	Porti     []models.Porto
	Nave      models.Nave
	Compagnie []models.Compagnia
}

func ListaNavi(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Navi - FurvioGest", r)
	// Controlla path esatto
	if r.URL.Path != "/navi" && r.URL.Path != "/navi/" {
		http.NotFound(w, r)
		return
	}

	rows, err := database.DB.Query(`
		SELECT n.id, n.compagnia_id, n.nome, n.imo, COALESCE(n.sigla, '') as sigla, n.email_master, n.email_direttore_macchina,
		       n.email_ispettore, n.note, n.ferma_per_lavori, n.data_inizio_lavori, 
		       n.data_fine_lavori_prevista, n.created_at, COALESCE(n.foto, '') as foto, c.nome as nome_compagnia, 
		       c.id as cid, COALESCE(c.logo, '') as logo
		FROM navi n
		JOIN compagnie c ON n.compagnia_id = c.id
		ORDER BY c.nome, n.nome
	`)
	if err != nil {
		data.Error = "Errore nel recupero delle navi"
		renderTemplate(w, "navi_lista.html", data)
		return
	}
	defer rows.Close()

	// Struct per compagnia con info complete
	type CompagniaInfo struct {
		ID   int64
		Nome string
		Logo string
	}

	// Mappa per raggruppare le navi per compagnia
	naviPerCompagnia := make(map[int64][]models.Nave)
	compagniaInfo := make(map[int64]CompagniaInfo)
	// Slice per mantenere l'ordine delle compagnie
	var ordineCompagnie []int64
	compagnieViste := make(map[int64]bool)

	for rows.Next() {
		var n models.Nave
		var imo, emailMaster, emailDirettore, emailIspettore, note sql.NullString
		var dataInizioLavori, dataFineLavori sql.NullTime
		var compagniaID int64
		var compagniaNome, compagniaLogo string
		
		err := rows.Scan(&n.ID, &n.CompagniaID, &n.Nome, &imo, &n.Sigla, &emailMaster, &emailDirettore,
			&emailIspettore, &note, &n.FermaPerLavori, &dataInizioLavori, &dataFineLavori,
			&n.CreatedAt, &n.Foto, &n.NomeCompagnia, &compagniaID, &compagniaLogo)
		if err != nil {
			continue
		}
		compagniaNome = n.NomeCompagnia
		
		if imo.Valid { n.IMO = imo.String }
		if emailMaster.Valid { n.EmailMaster = emailMaster.String }
		if emailDirettore.Valid { n.EmailDirettoreMacchina = emailDirettore.String }
		if emailIspettore.Valid { n.EmailIspettore = emailIspettore.String }
		if note.Valid { n.Note = note.String }
		if dataInizioLavori.Valid { n.DataInizioLavori = &dataInizioLavori.Time }
		if dataFineLavori.Valid { n.DataFineLavoriPrev = &dataFineLavori.Time }

		// Aggiungi alla mappa per compagnia
		naviPerCompagnia[compagniaID] = append(naviPerCompagnia[compagniaID], n)
		
		// Mantieni ordine e info compagnie
		if !compagnieViste[compagniaID] {
			ordineCompagnie = append(ordineCompagnie, compagniaID)
			compagnieViste[compagniaID] = true
			compagniaInfo[compagniaID] = CompagniaInfo{
				ID:   compagniaID,
				Nome: compagniaNome,
				Logo: compagniaLogo,
			}
		}
	}

	// Crea una slice ordinata per il template
	type CompagniaNavi struct {
		ID   int64
		Nome string
		Logo string
		Navi []models.Nave
	}
	var naviOrdinate []CompagniaNavi
	for _, cid := range ordineCompagnie {
		info := compagniaInfo[cid]
		naviOrdinate = append(naviOrdinate, CompagniaNavi{
			ID:   info.ID,
			Nome: info.Nome,
			Logo: info.Logo,
			Navi: naviPerCompagnia[cid],
		})
	}

	data.Data = naviOrdinate
	renderTemplate(w, "navi_lista.html", data)
}

func getPorti() []models.Porto {
	var porti []models.Porto
	rows, err := database.DB.Query("SELECT id, nome FROM porti ORDER BY nome")
	if err != nil {
		return porti
	}
	defer rows.Close()

	for rows.Next() {
		var p models.Porto
		rows.Scan(&p.ID, &p.Nome)
		porti = append(porti, p)
	}
	return porti
}

func getCompagnie() []models.Compagnia {
	var compagnie []models.Compagnia
	rows, err := database.DB.Query("SELECT id, nome FROM compagnie ORDER BY nome")
	if err != nil {
		return compagnie
	}
	defer rows.Close()

	for rows.Next() {
		var c models.Compagnia
		rows.Scan(&c.ID, &c.Nome)
		compagnie = append(compagnie, c)
	}
	return compagnie
}

func NuovaNave(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Nuova Nave - FurvioGest", r)

	formData := NaveFormData{
		Compagnie: getCompagnie(),
		Porti:     getPorti(),
	}

	if r.Method == http.MethodGet {
		data.Data = formData
		renderTemplate(w, "navi_form.html", data)
		return
	}

	r.ParseMultipartForm(10 << 20)
	compagniaID, _ := strconv.ParseInt(r.FormValue("compagnia_id"), 10, 64)
	nome := strings.TrimSpace(r.FormValue("nome"))
	imo := strings.TrimSpace(r.FormValue("imo"))
	emailMaster := strings.TrimSpace(r.FormValue("email_master"))
	emailDirettore := strings.TrimSpace(r.FormValue("email_direttore_macchina"))
	emailIspettore := strings.TrimSpace(r.FormValue("email_ispettore"))
	telMaster := strings.TrimSpace(r.FormValue("tel_master"))
	telDirettore := strings.TrimSpace(r.FormValue("tel_direttore_macchina"))
	telIspettore := strings.TrimSpace(r.FormValue("tel_ispettore"))
	note := strings.TrimSpace(r.FormValue("note"))
	sigla := strings.TrimSpace(r.FormValue("sigla"))
	emailDestinatari := r.FormValue("email_destinatari")
	if emailDestinatari == "" {
		emailDestinatari = "solo_agenzia"
	}
	fermaPerLavori := r.FormValue("ferma_per_lavori") == "on"

	var dataInizioLavori, dataFineLavoriPrev interface{}
	if fermaPerLavori {
		if d := r.FormValue("data_inizio_lavori"); d != "" {
			dataInizioLavori = d
		}
		if d := r.FormValue("data_fine_lavori_prevista"); d != "" {
			dataFineLavoriPrev = d
		}
	}

	if nome == "" || compagniaID == 0 {
		data.Error = "Nome e compagnia sono obbligatori"
		data.Data = formData
		renderTemplate(w, "navi_form.html", data)
		return
	}

	_, err := database.DB.Exec(`
		INSERT INTO navi (compagnia_id, nome, imo, email_master, email_direttore_macchina, sigla,
		                  email_ispettore, tel_master, tel_direttore_macchina, tel_ispettore,
		                  note, ferma_per_lavori, data_inizio_lavori, data_fine_lavori_prevista)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, compagniaID, nome, imo, sigla, emailMaster, emailDirettore, emailIspettore,
	   telMaster, telDirettore, telIspettore, note, fermaPerLavori, dataInizioLavori, dataFineLavoriPrev)

	if err != nil {
		data.Error = "Errore durante il salvataggio"
		data.Data = formData
		renderTemplate(w, "navi_form.html", data)
		return
	}


	// Gestione upload foto
	var lastID int64
	database.DB.QueryRow("SELECT last_insert_rowid()").Scan(&lastID)
	file, header, fotoErr := r.FormFile("foto")
	if fotoErr == nil && header.Size > 0 {
		defer file.Close()
		fotoPath := saveNaveFoto(lastID, file, header)
		if fotoPath != "" {
			database.DB.Exec("UPDATE navi SET foto = ? WHERE id = ?", fotoPath, lastID)
		}
	}

	http.Redirect(w, r, "/navi", http.StatusSeeOther)
}

func ModificaNave(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Modifica Nave - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	formData := NaveFormData{
		Compagnie: getCompagnie(),
		Porti:     getPorti(),
	}

	if r.Method == http.MethodGet {
		var n models.Nave
		var imo, emailMaster, emailDirettore, emailIspettore, note sql.NullString
		var telMaster, telDirettore, telIspettore sql.NullString
		var dataInizioLavori, dataFineLavori sql.NullTime
		err := database.DB.QueryRow(`
			SELECT id, compagnia_id, nome, imo, COALESCE(sigla, '') as sigla, email_master, email_direttore_macchina,
			       email_ispettore, tel_master, tel_direttore_macchina, tel_ispettore,
			       note, ferma_per_lavori, data_inizio_lavori, data_fine_lavori_prevista
			FROM navi WHERE id = ?
		`, id).Scan(&n.ID, &n.CompagniaID, &n.Nome, &imo, &n.Sigla, &emailMaster, &emailDirettore,
		            &emailIspettore, &telMaster, &telDirettore, &telIspettore,
		            &note, &n.FermaPerLavori, &dataInizioLavori, &dataFineLavori)

		if err != nil {
			http.Redirect(w, r, "/navi", http.StatusSeeOther)
			return
		}
		if imo.Valid {
			n.IMO = imo.String
		}
		if emailMaster.Valid {
			n.EmailMaster = emailMaster.String
		}
		if emailDirettore.Valid {
			n.EmailDirettoreMacchina = emailDirettore.String
		}
		if emailIspettore.Valid {
			n.EmailIspettore = emailIspettore.String
		}
		if telMaster.Valid {
			n.TelMaster = telMaster.String
		}
		if telDirettore.Valid {
			n.TelDirettoreMacchina = telDirettore.String
		}
		if telIspettore.Valid {
			n.TelIspettore = telIspettore.String
		}
		if note.Valid {
			n.Note = note.String
		}
		if dataInizioLavori.Valid {
			n.DataInizioLavori = &dataInizioLavori.Time
		}
		if dataFineLavori.Valid {
			n.DataFineLavoriPrev = &dataFineLavori.Time
		}

		formData.Nave = n
		data.Data = formData
		renderTemplate(w, "navi_form.html", data)
		return
	}

	r.ParseMultipartForm(10 << 20)
	compagniaID, _ := strconv.ParseInt(r.FormValue("compagnia_id"), 10, 64)
	nome := strings.TrimSpace(r.FormValue("nome"))
	imo := strings.TrimSpace(r.FormValue("imo"))
	emailMaster := strings.TrimSpace(r.FormValue("email_master"))
	emailDirettore := strings.TrimSpace(r.FormValue("email_direttore_macchina"))
	emailIspettore := strings.TrimSpace(r.FormValue("email_ispettore"))
	telMaster := strings.TrimSpace(r.FormValue("tel_master"))
	telDirettore := strings.TrimSpace(r.FormValue("tel_direttore_macchina"))
	telIspettore := strings.TrimSpace(r.FormValue("tel_ispettore"))
	note := strings.TrimSpace(r.FormValue("note"))
	sigla := strings.TrimSpace(r.FormValue("sigla"))
	emailDestinatari := r.FormValue("email_destinatari")
	if emailDestinatari == "" {
		emailDestinatari = "solo_agenzia"
	}
	fermaPerLavori := r.FormValue("ferma_per_lavori") == "on"

	var dataInizioLavori, dataFineLavoriPrev interface{}
	if fermaPerLavori {
		if d := r.FormValue("data_inizio_lavori"); d != "" {
			dataInizioLavori = d
		}
		if d := r.FormValue("data_fine_lavori_prevista"); d != "" {
			dataFineLavoriPrev = d
		}
	}

	if nome == "" || compagniaID == 0 {
		data.Error = "Nome e compagnia sono obbligatori"
		data.Data = formData
		renderTemplate(w, "navi_form.html", data)
		return
	}

	_, err = database.DB.Exec(`
		UPDATE navi SET compagnia_id = ?, nome = ?, imo = ?, sigla = ?, email_master = ?,
		       email_direttore_macchina = ?, email_ispettore = ?,
		       tel_master = ?, tel_direttore_macchina = ?, tel_ispettore = ?,
		       note = ?, ferma_per_lavori = ?, data_inizio_lavori = ?, data_fine_lavori_prevista = ?,
		       updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, compagniaID, nome, imo, sigla, emailMaster, emailDirettore, emailIspettore,
	   telMaster, telDirettore, telIspettore, note, fermaPerLavori, dataInizioLavori, dataFineLavoriPrev, id)

	if err != nil {
		data.Error = "Errore durante il salvataggio"
		data.Data = formData
		renderTemplate(w, "navi_form.html", data)
		return
	}


	// Gestione upload foto
	file, header, fotoErr := r.FormFile("foto")
	if fotoErr == nil && header.Size > 0 {
		defer file.Close()
		fotoPath := saveNaveFoto(id, file, header)
		if fotoPath != "" {
			database.DB.Exec("UPDATE navi SET foto = ? WHERE id = ?", fotoPath, id)
		}
	}

	http.Redirect(w, r, "/navi", http.StatusSeeOther)
}

func EliminaNave(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	id, _ := strconv.ParseInt(pathParts[3], 10, 64)
	database.DB.Exec("DELETE FROM navi WHERE id = ?", id)
	http.Redirect(w, r, "/navi", http.StatusSeeOther)
}

// saveNaveFoto salva la foto di una nave
func saveNaveFoto(naveID int64, file multipart.File, header *multipart.FileHeader) string {
	uploadDir := filepath.Join("uploads", "navi")
	os.MkdirAll(uploadDir, 0755)

	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("foto_%d%s", naveID, ext)
	filePath := filepath.Join(uploadDir, filename)

	dst, err := os.Create(filePath)
	if err != nil {
		return ""
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		return ""
	}

	return filePath
}

// ServeNaveFoto serve la foto di una nave
func ServeNaveFoto(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var fotoPath sql.NullString
	database.DB.QueryRow("SELECT foto FROM navi WHERE id = ?", id).Scan(&fotoPath)

	if !fotoPath.Valid || fotoPath.String == "" {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, fotoPath.String)
}

// Funzioni helper per uploads
func saveUploadedFile(r *http.Request, fieldName, destDir string) (string, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Crea directory se non esiste
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	// Genera nome file unico
	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	destPath := filepath.Join(destDir, filename)

	// Crea file destinazione
	dst, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	// Copia contenuto
	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}

	return destPath, nil
}

// APIVerificaPIVA verifica una partita IVA tramite VIES (EU VAT validation)
func APIVerificaPIVA(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	countryCode := r.URL.Query().Get("country")
	vatNumber := r.URL.Query().Get("vat")
	
	if countryCode == "" || vatNumber == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid": false,
			"error": "Parametri mancanti",
		})
		return
	}
	
	// Prepara la richiesta SOAP per VIES
	soapRequest := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:tns1="urn:ec.europa.eu:taxud:vies:services:checkVat:types">
  <soap:Body>
    <tns1:checkVat>
      <tns1:countryCode>%s</tns1:countryCode>
      <tns1:vatNumber>%s</tns1:vatNumber>
    </tns1:checkVat>
  </soap:Body>
</soap:Envelope>`, countryCode, vatNumber)
	
	// Chiamata al servizio VIES
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(
		"https://ec.europa.eu/taxation_customs/vies/services/checkVatService",
		"text/xml; charset=utf-8",
		strings.NewReader(soapRequest),
	)
	
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid": false,
			"error": "Errore connessione VIES: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid": false,
			"error": "Errore lettura risposta",
		})
		return
	}
	
	bodyStr := string(body)
	
	// Parse semplice della risposta XML
	valid := strings.Contains(bodyStr, "<valid>true</valid>")
	
	result := map[string]interface{}{
		"valid": valid,
	}
	
	if valid {
		// Estrai nome azienda
		if nameStart := strings.Index(bodyStr, "<name>"); nameStart != -1 {
			nameEnd := strings.Index(bodyStr[nameStart:], "</name>")
			if nameEnd != -1 {
				name := bodyStr[nameStart+6 : nameStart+nameEnd]
				name = strings.TrimSpace(name)
				if name != "---" && name != "" {
					result["name"] = name
				}
			}
		}
		
		// Estrai indirizzo
		if addrStart := strings.Index(bodyStr, "<address>"); addrStart != -1 {
			addrEnd := strings.Index(bodyStr[addrStart:], "</address>")
			if addrEnd != -1 {
				address := bodyStr[addrStart+9 : addrStart+addrEnd]
				address = strings.TrimSpace(address)
				if address != "---" && address != "" {
					result["address"] = address
				}
			}
		}
	}
	
	json.NewEncoder(w).Encode(result)
}
