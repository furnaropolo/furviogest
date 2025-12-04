package handlers

import (
	"database/sql"
	"furviogest/internal/database"
	"furviogest/internal/middleware"
	"furviogest/internal/models"
	"net/http"
	"strconv"
	"strings"
	"time"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ============================================
// PRODOTTI
// ============================================

// ProdottoFormData contiene i dati per il form prodotto
type ProdottoFormData struct {
	Prodotto  models.Prodotto
	Fornitori []models.Fornitore
}

// ListaProdotti mostra la lista dei prodotti in magazzino
func ListaProdotti(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Magazzino - FurvioGest", r)

	// Filtri dalla query string
	filtroTipo := r.URL.Query().Get("tipo")
	filtroOrigine := r.URL.Query().Get("origine")
	filtroCategoria := r.URL.Query().Get("categoria")
	filtroBassa := r.URL.Query().Get("bassa_giacenza")

	query := `
		SELECT p.id, p.codice, p.nome, p.descrizione, p.categoria, p.tipo, p.origine,
		       p.fornitore_id, p.numero_fattura, p.data_fattura, p.nave_origine,
		       p.giacenza, p.giacenza_minima, p.unita_misura, p.note, p.created_at,
		       COALESCE(f.nome, '') as nome_fornitore
		FROM prodotti p
		LEFT JOIN fornitori f ON p.fornitore_id = f.id
		WHERE 1=1
	`
	var args []interface{}

	if filtroTipo != "" && (filtroTipo == "wifi" || filtroTipo == "gsm" || filtroTipo == "entrambi") {
		query += " AND (p.tipo = ? OR p.tipo = 'entrambi')"
		args = append(args, filtroTipo)
	}
	if filtroOrigine != "" && (filtroOrigine == "spare" || filtroOrigine == "nuovo") {
		query += " AND p.origine = ?"
		args = append(args, filtroOrigine)
	}
	if filtroCategoria != "" && (filtroCategoria == "materiale" || filtroCategoria == "cavo") {
		query += " AND p.categoria = ?"
		args = append(args, filtroCategoria)
	}
	if filtroBassa == "1" {
		query += " AND p.giacenza <= p.giacenza_minima"
	}

	query += " ORDER BY p.categoria, p.tipo, p.nome"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		data.Error = "Errore nel recupero dei prodotti"
		renderTemplate(w, "prodotti_lista.html", data)
		return
	}
	defer rows.Close()

	var prodotti []models.Prodotto
	for rows.Next() {
		var p models.Prodotto
		var descrizione, numeroFattura, naveOrigine, note sql.NullString
		var fornitoreID sql.NullInt64
		var dataFattura sql.NullTime

		err := rows.Scan(&p.ID, &p.Codice, &p.Nome, &descrizione, &p.Categoria, &p.Tipo, &p.Origine,
			&fornitoreID, &numeroFattura, &dataFattura, &naveOrigine,
			&p.Giacenza, &p.GiacenzaMinima, &p.UnitaMisura, &note, &p.CreatedAt, &p.NomeFornitore)
		if err != nil {
			continue
		}

		if descrizione.Valid {
			p.Descrizione = descrizione.String
		}
		if fornitoreID.Valid {
			p.FornitoreID = &fornitoreID.Int64
		}
		if numeroFattura.Valid {
			p.NumeroFattura = numeroFattura.String
		}
		if dataFattura.Valid {
			p.DataFattura = &dataFattura.Time
		}
		if naveOrigine.Valid {
			p.NaveOrigine = naveOrigine.String
		}
		if note.Valid {
			p.Note = note.String
		}

		prodotti = append(prodotti, p)
	}

	// Passa i filtri al template
	type ListaData struct {
		Prodotti        []models.Prodotto
		FiltroTipo      string
		FiltroOrigine   string
		FiltroCategoria string
		FiltroBassa     string
	}

	data.Data = ListaData{
		Prodotti:        prodotti,
		FiltroTipo:      filtroTipo,
		FiltroOrigine:   filtroOrigine,
		FiltroCategoria: filtroCategoria,
		FiltroBassa:     filtroBassa,
	}
	renderTemplate(w, "prodotti_lista.html", data)
}

// getFornitori recupera tutti i fornitori per le select
func getFornitori() []models.Fornitore {
	var fornitori []models.Fornitore
	rows, err := database.DB.Query("SELECT id, nome FROM fornitori ORDER BY nome")
	if err != nil {
		return fornitori
	}
	defer rows.Close()

	for rows.Next() {
		var f models.Fornitore
		rows.Scan(&f.ID, &f.Nome)
		fornitori = append(fornitori, f)
	}
	return fornitori
}

// NuovoProdotto gestisce la creazione di un nuovo prodotto
func NuovoProdotto(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Nuovo Prodotto - FurvioGest", r)

	formData := ProdottoFormData{
		Fornitori: getFornitori(),
	}

	if r.Method == http.MethodGet {
		data.Data = formData
		renderTemplate(w, "prodotti_form.html", data)
		return
	}

	r.ParseForm()
	codice := strings.ToUpper(strings.TrimSpace(r.FormValue("codice")))
	nome := strings.TrimSpace(r.FormValue("nome"))
	descrizione := strings.TrimSpace(r.FormValue("descrizione"))
	categoria := strings.TrimSpace(r.FormValue("categoria"))
	tipo := strings.TrimSpace(r.FormValue("tipo"))
	origine := strings.TrimSpace(r.FormValue("origine"))
	fornitoreIDStr := r.FormValue("fornitore_id")
	numeroFattura := strings.TrimSpace(r.FormValue("numero_fattura"))
	dataFatturaStr := r.FormValue("data_fattura")
	naveOrigine := strings.TrimSpace(r.FormValue("nave_origine"))
	giacenzaStr := r.FormValue("giacenza")
	giacenzaMinimaStr := r.FormValue("giacenza_minima")
	note := strings.TrimSpace(r.FormValue("note"))

	// Validazione
	if codice == "" || nome == "" {
		data.Error = "Codice e nome sono obbligatori"
		data.Data = formData
		renderTemplate(w, "prodotti_form.html", data)
		return
	}

	// Valida categoria
	if categoria != "materiale" && categoria != "cavo" {
		categoria = "materiale" // default
	}

	// Unità di misura dipende dalla categoria
	unitaMisura := "pz"
	if categoria == "cavo" {
		unitaMisura = "m"
	}

	if tipo != "wifi" && tipo != "gsm" && tipo != "entrambi" {
		data.Error = "Tipo deve essere 'wifi', 'gsm' o 'entrambi'"
		data.Data = formData
		renderTemplate(w, "prodotti_form.html", data)
		return
	}

	if origine != "spare" && origine != "nuovo" {
		data.Error = "Origine deve essere 'spare' o 'nuovo'"
		data.Data = formData
		renderTemplate(w, "prodotti_form.html", data)
		return
	}

	// Parsing valori numerici (float per supportare metri decimali)
	giacenza, _ := strconv.ParseFloat(giacenzaStr, 64)
	giacenzaMinima, _ := strconv.ParseFloat(giacenzaMinimaStr, 64)

	var fornitoreID *int64
	if fornitoreIDStr != "" {
		fid, err := strconv.ParseInt(fornitoreIDStr, 10, 64)
		if err == nil && fid > 0 {
			fornitoreID = &fid
		}
	}

	var dataFattura *time.Time
	if dataFatturaStr != "" {
		if t, err := time.Parse("2006-01-02", dataFatturaStr); err == nil {
			dataFattura = &t
		}
	}

	_, err := database.DB.Exec(`
		INSERT INTO prodotti (codice, nome, descrizione, categoria, tipo, origine, fornitore_id,
		                      numero_fattura, data_fattura, nave_origine, giacenza, giacenza_minima, unita_misura, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, codice, nome, descrizione, categoria, tipo, origine, fornitoreID, numeroFattura, dataFattura, naveOrigine, giacenza, giacenzaMinima, unitaMisura, note)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			data.Error = "Codice prodotto già esistente"
		} else {
			data.Error = "Errore durante il salvataggio: " + err.Error()
		}
		data.Data = formData
		renderTemplate(w, "prodotti_form.html", data)
		return
	}

	http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
}

// ModificaProdotto gestisce la modifica di un prodotto
func ModificaProdotto(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Modifica Prodotto - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
		return
	}

	formData := ProdottoFormData{
		Fornitori: getFornitori(),
	}

	if r.Method == http.MethodGet {
		var p models.Prodotto
		var descrizione, numeroFattura, naveOrigine, note sql.NullString
		var fornitoreID sql.NullInt64
		var dataFattura sql.NullTime

		err := database.DB.QueryRow(`
			SELECT id, codice, nome, descrizione, categoria, tipo, origine, fornitore_id,
			       numero_fattura, data_fattura, nave_origine, giacenza, giacenza_minima, unita_misura, note
			FROM prodotti WHERE id = ?
		`, id).Scan(&p.ID, &p.Codice, &p.Nome, &descrizione, &p.Categoria, &p.Tipo, &p.Origine,
			&fornitoreID, &numeroFattura, &dataFattura, &naveOrigine,
			&p.Giacenza, &p.GiacenzaMinima, &p.UnitaMisura, &note)

		if err != nil {
			http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
			return
		}

		if descrizione.Valid {
			p.Descrizione = descrizione.String
		}
		if fornitoreID.Valid {
			p.FornitoreID = &fornitoreID.Int64
		}
		if numeroFattura.Valid {
			p.NumeroFattura = numeroFattura.String
		}
		if dataFattura.Valid {
			p.DataFattura = &dataFattura.Time
		}
		if naveOrigine.Valid {
			p.NaveOrigine = naveOrigine.String
		}
		if note.Valid {
			p.Note = note.String
		}

		formData.Prodotto = p
		data.Data = formData
		renderTemplate(w, "prodotti_form.html", data)
		return
	}

	r.ParseForm()
	codice := strings.ToUpper(strings.TrimSpace(r.FormValue("codice")))
	nome := strings.TrimSpace(r.FormValue("nome"))
	descrizione := strings.TrimSpace(r.FormValue("descrizione"))
	categoria := strings.TrimSpace(r.FormValue("categoria"))
	tipo := strings.TrimSpace(r.FormValue("tipo"))
	origine := strings.TrimSpace(r.FormValue("origine"))
	fornitoreIDStr := r.FormValue("fornitore_id")
	numeroFattura := strings.TrimSpace(r.FormValue("numero_fattura"))
	dataFatturaStr := r.FormValue("data_fattura")
	naveOrigine := strings.TrimSpace(r.FormValue("nave_origine"))
	giacenzaMinimaStr := r.FormValue("giacenza_minima")
	note := strings.TrimSpace(r.FormValue("note"))

	// Validazione
	if codice == "" || nome == "" {
		data.Error = "Codice e nome sono obbligatori"
		data.Data = formData
		renderTemplate(w, "prodotti_form.html", data)
		return
	}

	// Valida categoria
	if categoria != "materiale" && categoria != "cavo" {
		categoria = "materiale"
	}

	// Unità di misura dipende dalla categoria
	unitaMisura := "pz"
	if categoria == "cavo" {
		unitaMisura = "m"
	}

	giacenzaMinima, _ := strconv.ParseFloat(giacenzaMinimaStr, 64)

	var fornitoreID *int64
	if fornitoreIDStr != "" {
		fid, err := strconv.ParseInt(fornitoreIDStr, 10, 64)
		if err == nil && fid > 0 {
			fornitoreID = &fid
		}
	}

	var dataFattura *time.Time
	if dataFatturaStr != "" {
		if t, err := time.Parse("2006-01-02", dataFatturaStr); err == nil {
			dataFattura = &t
		}
	}

	_, err = database.DB.Exec(`
		UPDATE prodotti SET codice = ?, nome = ?, descrizione = ?, categoria = ?, tipo = ?, origine = ?,
		       fornitore_id = ?, numero_fattura = ?, data_fattura = ?, nave_origine = ?,
		       giacenza_minima = ?, unita_misura = ?, note = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, codice, nome, descrizione, categoria, tipo, origine, fornitoreID, numeroFattura, dataFattura, naveOrigine, giacenzaMinima, unitaMisura, note, id)

	if err != nil {
		data.Error = "Errore durante il salvataggio"
		data.Data = formData
		renderTemplate(w, "prodotti_form.html", data)
		return
	}

	http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
}

// EliminaProdotto elimina un prodotto
func EliminaProdotto(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
		return
	}

	id, _ := strconv.ParseInt(pathParts[3], 10, 64)
	database.DB.Exec("DELETE FROM prodotti WHERE id = ?", id)
	http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
}

// ============================================
// MOVIMENTI MAGAZZINO
// ============================================

// MovimentoFormData contiene i dati per il form movimento
type MovimentoFormData struct {
	Prodotto  models.Prodotto
	Movimento models.MovimentoMagazzino
}

// ListaMovimenti mostra lo storico movimenti di un prodotto
func ListaMovimenti(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Movimenti Magazzino - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
		return
	}

	prodottoID, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
		return
	}

	// Recupera info prodotto
	var prodotto models.Prodotto
	err = database.DB.QueryRow(`
		SELECT id, codice, nome, categoria, giacenza, unita_misura FROM prodotti WHERE id = ?
	`, prodottoID).Scan(&prodotto.ID, &prodotto.Codice, &prodotto.Nome, &prodotto.Categoria, &prodotto.Giacenza, &prodotto.UnitaMisura)
	if err != nil {
		http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
		return
	}

	// Recupera movimenti
	rows, err := database.DB.Query(`
		SELECT m.id, m.prodotto_id, m.tecnico_id, m.quantita, m.tipo, m.motivo, m.created_at,
		       u.nome || ' ' || u.cognome as nome_tecnico
		FROM movimenti_magazzino m
		JOIN utenti u ON m.tecnico_id = u.id
		WHERE m.prodotto_id = ?
		ORDER BY m.created_at DESC
	`, prodottoID)
	if err != nil {
		data.Error = "Errore nel recupero dei movimenti"
		renderTemplate(w, "movimenti_lista.html", data)
		return
	}
	defer rows.Close()

	var movimenti []models.MovimentoMagazzino
	for rows.Next() {
		var m models.MovimentoMagazzino
		var motivo sql.NullString

		err := rows.Scan(&m.ID, &m.ProdottoID, &m.TecnicoID, &m.Quantita, &m.Tipo, &motivo, &m.CreatedAt, &m.NomeTecnico)
		if err != nil {
			continue
		}

		if motivo.Valid {
			m.Motivo = motivo.String
		}
		m.UnitaMisura = prodotto.UnitaMisura

		movimenti = append(movimenti, m)
	}

	type MovimentiData struct {
		Prodotto  models.Prodotto
		Movimenti []models.MovimentoMagazzino
	}

	data.Data = MovimentiData{
		Prodotto:  prodotto,
		Movimenti: movimenti,
	}
	renderTemplate(w, "movimenti_lista.html", data)
}

// NuovoMovimento gestisce l'inserimento di un movimento (carico/scarico)
func NuovoMovimento(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Nuovo Movimento - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
		return
	}

	prodottoID, err := strconv.ParseInt(pathParts[4], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
		return
	}

	// Recupera info prodotto
	var prodotto models.Prodotto
	err = database.DB.QueryRow(`
		SELECT id, codice, nome, categoria, giacenza, unita_misura FROM prodotti WHERE id = ?
	`, prodottoID).Scan(&prodotto.ID, &prodotto.Codice, &prodotto.Nome, &prodotto.Categoria, &prodotto.Giacenza, &prodotto.UnitaMisura)
	if err != nil {
		http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
		return
	}

	formData := MovimentoFormData{
		Prodotto: prodotto,
	}

	if r.Method == http.MethodGet {
		data.Data = formData
		renderTemplate(w, "movimenti_form.html", data)
		return
	}

	// Parse multipart form per upload file
	err = r.ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil {
		r.ParseForm()
	}

	tipoMov := strings.TrimSpace(r.FormValue("tipo"))
	quantitaStr := r.FormValue("quantita")
	motivo := strings.TrimSpace(r.FormValue("motivo"))

	quantita, _ := strconv.ParseFloat(quantitaStr, 64)

	// Validazione
	if quantita <= 0 {
		data.Error = "La quantità deve essere maggiore di zero"
		data.Data = formData
		renderTemplate(w, "movimenti_form.html", data)
		return
	}

	if tipoMov != "carico" && tipoMov != "scarico" {
		data.Error = "Tipo movimento non valido"
		data.Data = formData
		renderTemplate(w, "movimenti_form.html", data)
		return
	}

	// Per lo scarico, verifica disponibilità
	if tipoMov == "scarico" && quantita > prodotto.Giacenza {
		data.Error = "Quantità non disponibile in magazzino"
		data.Data = formData
		renderTemplate(w, "movimenti_form.html", data)
		return
	}

	// Recupera ID tecnico dalla sessione
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Gestione upload documento (solo per carico)
	var documentoPath string
	if tipoMov == "carico" {
		file, header, err := r.FormFile("documento")
		if err == nil && header != nil {
			defer file.Close()
			
			// Verifica estensione
			ext := strings.ToLower(filepath.Ext(header.Filename))
			if ext == ".pdf" {
				// Crea directory per documenti magazzino
				uploadsDir := "web/static/uploads/magazzino"
				os.MkdirAll(uploadsDir, 0755)
				
				// Nome file univoco
				filename := fmt.Sprintf("mov_%d_%d%s", prodottoID, time.Now().Unix(), ext)
				destPath := filepath.Join(uploadsDir, filename)
				
				// Salva file
				destFile, err := os.Create(destPath)
				if err == nil {
					defer destFile.Close()
					io.Copy(destFile, file)
					documentoPath = "/static/uploads/magazzino/" + filename
				}
			}
		}
	}

	// Inizia transazione
	tx, err := database.DB.Begin()
	if err != nil {
		data.Error = "Errore durante il salvataggio"
		data.Data = formData
		renderTemplate(w, "movimenti_form.html", data)
		return
	}

	// Inserisce il movimento con eventuale documento
	_, err = tx.Exec(`
		INSERT INTO movimenti_magazzino (prodotto_id, tecnico_id, quantita, tipo, motivo, documento_path)
		VALUES (?, ?, ?, ?, ?, ?)
	`, prodottoID, session.UserID, quantita, tipoMov, motivo, documentoPath)
	if err != nil {
		tx.Rollback()
		data.Error = "Errore durante il salvataggio"
		data.Data = formData
		renderTemplate(w, "movimenti_form.html", data)
		return
	}

	// Aggiorna la giacenza
	var deltaGiacenza float64
	if tipoMov == "carico" {
		deltaGiacenza = quantita
	} else {
		deltaGiacenza = -quantita
	}

	_, err = tx.Exec(`
		UPDATE prodotti SET giacenza = giacenza + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, deltaGiacenza, prodottoID)
	if err != nil {
		tx.Rollback()
		data.Error = "Errore durante l'aggiornamento giacenza"
		data.Data = formData
		renderTemplate(w, "movimenti_form.html", data)
		return
	}

	tx.Commit()

	http.Redirect(w, r, "/magazzino/movimenti/"+strconv.FormatInt(prodottoID, 10), http.StatusSeeOther)
}
