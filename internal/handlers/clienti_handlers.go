package handlers

import (
	"database/sql"
	"furviogest/internal/database"
	"furviogest/internal/models"
	"net/http"
	"strconv"
	"strings"
)

// ============================================
// CLIENTI
// ============================================

func ListaClienti(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Clienti - FurvioGest", r)

	// Gestione messaggi di errore da query string
	errorType := r.URL.Query().Get("error")
	switch errorType {
	case "ddt_collegati":
		data.Error = "Impossibile eliminare il cliente: esistono DDT collegati. Eliminare prima i DDT associati."
	case "delete":
		data.Error = "Errore durante l'eliminazione del cliente"
	}

	rows, err := database.DB.Query(`
		SELECT id, nome, COALESCE(partita_iva,''), COALESCE(codice_fiscale,''), 
		       COALESCE(indirizzo,''), COALESCE(cap,''), COALESCE(citta,''), 
		       COALESCE(provincia,''), COALESCE(nazione,'Italia'), COALESCE(telefono,''), 
		       COALESCE(cellulare,''), COALESCE(email,''), COALESCE(referente,''), 
		       COALESCE(telefono_referente,''), COALESCE(note,''), created_at
		FROM clienti ORDER BY nome
	`)
	if err != nil {
		data.Error = "Errore nel recupero dei clienti"
		renderTemplate(w, "clienti_lista.html", data)
		return
	}
	defer rows.Close()

	var clienti []models.Cliente
	for rows.Next() {
		var c models.Cliente
		err := rows.Scan(&c.ID, &c.Nome, &c.PartitaIVA, &c.CodiceFiscale, 
			&c.Indirizzo, &c.CAP, &c.Citta, &c.Provincia, &c.Nazione, 
			&c.Telefono, &c.Cellulare, &c.Email, &c.Referente, 
			&c.TelefonoReferente, &c.Note, &c.CreatedAt)
		if err != nil {
			continue
		}
		clienti = append(clienti, c)
	}

	data.Data = clienti
	renderTemplate(w, "clienti_lista.html", data)
}

func NuovoCliente(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Nuovo Cliente - FurvioGest", r)

	if r.Method == http.MethodGet {
		renderTemplate(w, "clienti_form.html", data)
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
		renderTemplate(w, "clienti_form.html", data)
		return
	}

	_, err := database.DB.Exec(`
		INSERT INTO clienti (nome, partita_iva, codice_fiscale, indirizzo, cap, citta, provincia, nazione, telefono, cellulare, email, referente, telefono_referente, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, nome, partitaIVA, codiceFiscale, indirizzo, cap, citta, provincia, nazione, telefono, cellulare, email, referente, telefonoReferente, note)

	if err != nil {
		data.Error = "Errore durante il salvataggio: " + err.Error()
		renderTemplate(w, "clienti_form.html", data)
		return
	}

	http.Redirect(w, r, "/clienti", http.StatusSeeOther)
}

func ModificaCliente(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Modifica Cliente - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/clienti", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/clienti", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		var c models.Cliente
		err := database.DB.QueryRow(`
			SELECT id, nome, COALESCE(partita_iva,''), COALESCE(codice_fiscale,''), 
			       COALESCE(indirizzo,''), COALESCE(cap,''), COALESCE(citta,''), 
			       COALESCE(provincia,''), COALESCE(nazione,'Italia'), COALESCE(telefono,''), 
			       COALESCE(cellulare,''), COALESCE(email,''), COALESCE(referente,''), 
			       COALESCE(telefono_referente,''), COALESCE(note,'')
			FROM clienti WHERE id = ?
		`, id).Scan(&c.ID, &c.Nome, &c.PartitaIVA, &c.CodiceFiscale,
			&c.Indirizzo, &c.CAP, &c.Citta, &c.Provincia, &c.Nazione,
			&c.Telefono, &c.Cellulare, &c.Email, &c.Referente,
			&c.TelefonoReferente, &c.Note)

		if err == sql.ErrNoRows {
			http.Redirect(w, r, "/clienti", http.StatusSeeOther)
			return
		}
		if err != nil {
			data.Error = "Errore nel recupero del cliente"
			renderTemplate(w, "clienti_form.html", data)
			return
		}

		data.Data = c
		renderTemplate(w, "clienti_form.html", data)
		return
	}

	// POST - salva modifiche
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
		renderTemplate(w, "clienti_form.html", data)
		return
	}

	_, err = database.DB.Exec(`
		UPDATE clienti SET nome=?, partita_iva=?, codice_fiscale=?, indirizzo=?, cap=?, citta=?, provincia=?, nazione=?, telefono=?, cellulare=?, email=?, referente=?, telefono_referente=?, note=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?
	`, nome, partitaIVA, codiceFiscale, indirizzo, cap, citta, provincia, nazione, telefono, cellulare, email, referente, telefonoReferente, note, id)

	if err != nil {
		data.Error = "Errore durante il salvataggio: " + err.Error()
		renderTemplate(w, "clienti_form.html", data)
		return
	}

	http.Redirect(w, r, "/clienti", http.StatusSeeOther)
}

func EliminaCliente(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/clienti", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/clienti", http.StatusSeeOther)
		return
	}

	// Verifica se ci sono DDT collegati
	var count int
	err = database.DB.QueryRow("SELECT COUNT(*) FROM ddt_uscita WHERE cliente_id = ?", id).Scan(&count)
	if err == nil && count > 0 {
		// Non eliminare, ci sono DDT collegati
		http.Redirect(w, r, "/clienti?error=ddt_collegati", http.StatusSeeOther)
		return
	}

	_, err = database.DB.Exec("DELETE FROM clienti WHERE id = ?", id)
	if err != nil {
		http.Redirect(w, r, "/clienti?error=delete", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/clienti", http.StatusSeeOther)
}
