package handlers

import (
	"database/sql"
	"encoding/json"
	"furviogest/internal/database"
	"furviogest/internal/models"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ============================================
// PRODOTTI
// ============================================

// ProdottoFormData contiene i dati per il form prodotto
type ProdottoFormData struct {
	Prodotto   models.Prodotto
	Fornitori  []FornitoreSelect
	DDTFatture []DDTFatturaSelect
	Navi       []NaveSelect
}

// NaveSelect per select navi
type NaveSelect struct {
	ID   int64
	Nome string
}

// DDTFatturaSelect per select DDT/Fatture
type DDTFatturaSelect struct {
	ID            int64
	FornitoreID   int64
	Tipo          string
	Numero        string
	DataDocumento time.Time
	NomeFornitore string
	IsAmazon      bool
	Label         string
}

// MovimentoAcquisto rappresenta un acquisto collegato ad un prodotto
type MovimentoAcquisto struct {
	ID            int64
	ProdottoID    int64
	DDTFatturaID  int64
	Quantita      int
	Note          string
	CreatedAt     time.Time
	// Campi virtuali
	TipoDoc       string
	NumeroDoc     string
	DataDoc       time.Time
	NomeFornitore string
	IsAmazon      bool
}

// ListaProdotti mostra la lista dei prodotti in magazzino
func ListaProdotti(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Magazzino - FurvioGest", r)

	// Filtri dalla query string
	filtroTipo := r.URL.Query().Get("tipo")
	filtroOrigine := r.URL.Query().Get("origine")
	filtroCategoria := r.URL.Query().Get("categoria")

	query := `
		SELECT p.id, p.codice, p.nome, p.descrizione, p.categoria, p.tipo, p.origine,
		       p.nave_origine, p.giacenza, p.unita_misura, p.note, p.created_at
		FROM prodotti p
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
		var descrizione, naveOrigine, note sql.NullString

		err := rows.Scan(&p.ID, &p.Codice, &p.Nome, &descrizione, &p.Categoria, &p.Tipo, &p.Origine,
			&naveOrigine, &p.Giacenza, &p.UnitaMisura, &note, &p.CreatedAt)
		if err != nil {
			continue
		}

		if descrizione.Valid {
			p.Descrizione = descrizione.String
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
	}

	data.Data = ListaData{
		Prodotti:        prodotti,
		FiltroTipo:      filtroTipo,
		FiltroOrigine:   filtroOrigine,
		FiltroCategoria: filtroCategoria,
	}
	renderTemplate(w, "prodotti_lista.html", data)
}

// caricaNavi recupera tutte le navi per select
func caricaNaviMagazzino() []NaveSelect {
	var navi []NaveSelect
	rows, err := database.DB.Query("SELECT id, nome FROM navi ORDER BY nome")
	if err != nil {
		return navi
	}
	defer rows.Close()

	for rows.Next() {
		var n NaveSelect
		rows.Scan(&n.ID, &n.Nome)
		navi = append(navi, n)
	}
	return navi
}

// caricaDDTFatture recupera DDT/Fatture per select
func caricaDDTFatture() []DDTFatturaSelect {
	var ddts []DDTFatturaSelect
	rows, err := database.DB.Query(`
		SELECT d.id, d.fornitore_id, d.tipo, d.numero, d.data_documento, 
		       f.nome, COALESCE(f.is_amazon, 0)
		FROM ddt_fatture d
		LEFT JOIN fornitori f ON d.fornitore_id = f.id
		ORDER BY d.data_documento DESC
	`)
	if err != nil {
		return ddts
	}
	defer rows.Close()

	for rows.Next() {
		var d DDTFatturaSelect
		rows.Scan(&d.ID, &d.FornitoreID, &d.Tipo, &d.Numero, &d.DataDocumento, &d.NomeFornitore, &d.IsAmazon)
		
		tipoLabel := d.Tipo
		if d.IsAmazon {
			tipoLabel = "Ordine"
		} else if d.Tipo == "fattura" {
			tipoLabel = "Fattura"
		} else {
			tipoLabel = "DDT"
		}
		d.Label = tipoLabel + " n." + d.Numero + " del " + d.DataDocumento.Format("02/01/2006") + " - " + d.NomeFornitore
		
		ddts = append(ddts, d)
	}
	return ddts
}

// NuovoProdotto gestisce la creazione di un nuovo prodotto
func NuovoProdotto(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Nuovo Prodotto - FurvioGest", r)

	formData := ProdottoFormData{
		Fornitori:  caricaFornitoriConAmazon(),
		DDTFatture: caricaDDTFatture(),
		Navi:       caricaNaviMagazzino(),
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
	naveOrigine := strings.TrimSpace(r.FormValue("nave_origine"))
	ddtFatturaIDStr := r.FormValue("ddt_fattura_id")
	quantitaStr := r.FormValue("quantita")
	note := strings.TrimSpace(r.FormValue("note"))

	// Validazione base
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

	// Parsing quantità
	quantita, _ := strconv.Atoi(quantitaStr)
	if quantita < 0 {
		quantita = 0
	}

	// Per prodotti NON spare, è obbligatorio DDT/Fattura
	var ddtFatturaID int64
	if origine == "nuovo" {
		ddtFatturaID, _ = strconv.ParseInt(ddtFatturaIDStr, 10, 64)
		if ddtFatturaID == 0 {
			data.Error = "Per i prodotti nuovi è obbligatorio selezionare un DDT/Fattura di acquisto"
			data.Data = formData
			renderTemplate(w, "prodotti_form.html", data)
			return
		}
		if quantita <= 0 {
			data.Error = "Per i prodotti nuovi la quantità deve essere maggiore di 0"
			data.Data = formData
			renderTemplate(w, "prodotti_form.html", data)
			return
		}
	}

	// Per spare è obbligatoria la nave di origine
	if origine == "spare" && naveOrigine == "" {
		data.Error = "Per i prodotti spare è obbligatorio indicare la nave di origine"
		data.Data = formData
		renderTemplate(w, "prodotti_form.html", data)
		return
	}

	// Inizia transazione
	tx, err := database.DB.Begin()
	if err != nil {
		data.Error = "Errore database"
		data.Data = formData
		renderTemplate(w, "prodotti_form.html", data)
		return
	}

	// Inserisci prodotto
	var giacenzaIniziale int
	if origine == "spare" {
		giacenzaIniziale = quantita // Per spare la giacenza è diretta
	} else {
		giacenzaIniziale = quantita // Verrà anche registrata nel movimento
	}

	result, err := tx.Exec(`
		INSERT INTO prodotti (codice, nome, descrizione, categoria, tipo, origine, nave_origine, giacenza, unita_misura, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, codice, nome, descrizione, categoria, tipo, origine, naveOrigine, giacenzaIniziale, unitaMisura, note)

	if err != nil {
		tx.Rollback()
		if strings.Contains(err.Error(), "UNIQUE") {
			data.Error = "Codice prodotto già esistente"
		} else {
			data.Error = "Errore durante il salvataggio: " + err.Error()
		}
		data.Data = formData
		renderTemplate(w, "prodotti_form.html", data)
		return
	}

	prodottoID, _ := result.LastInsertId()

	// Per prodotti nuovi, crea movimento acquisto
	if origine == "nuovo" && ddtFatturaID > 0 {
		_, err = tx.Exec(`
			INSERT INTO movimenti_acquisto (prodotto_id, ddt_fattura_id, quantita)
			VALUES (?, ?, ?)
		`, prodottoID, ddtFatturaID, quantita)
		if err != nil {
			tx.Rollback()
			data.Error = "Errore creazione movimento: " + err.Error()
			data.Data = formData
			renderTemplate(w, "prodotti_form.html", data)
			return
		}
	}

	tx.Commit()
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
		Fornitori:  caricaFornitoriConAmazon(),
		DDTFatture: caricaDDTFatture(),
		Navi:       caricaNaviMagazzino(),
	}

	if r.Method == http.MethodGet {
		var p models.Prodotto
		var descrizione, naveOrigine, note sql.NullString

		err := database.DB.QueryRow(`
			SELECT id, codice, nome, descrizione, categoria, tipo, origine, nave_origine, giacenza, unita_misura, note
			FROM prodotti WHERE id = ?
		`, id).Scan(&p.ID, &p.Codice, &p.Nome, &descrizione, &p.Categoria, &p.Tipo, &p.Origine,
			&naveOrigine, &p.Giacenza, &p.UnitaMisura, &note)

		if err != nil {
			http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
			return
		}

		if descrizione.Valid {
			p.Descrizione = descrizione.String
		}
		if naveOrigine.Valid {
			p.NaveOrigine = naveOrigine.String
		}
		if note.Valid {
			p.Note = note.String
		}

		// Carica movimenti acquisto per questo prodotto
		movimenti := caricaMovimentiAcquisto(id)

		formData.Prodotto = p
		data.Data = map[string]interface{}{
			"FormData":  formData,
			"Movimenti": movimenti,
		}
		renderTemplate(w, "prodotti_form.html", data)
		return
	}

	r.ParseForm()
	codice := strings.ToUpper(strings.TrimSpace(r.FormValue("codice")))
	nome := strings.TrimSpace(r.FormValue("nome"))
	descrizione := strings.TrimSpace(r.FormValue("descrizione"))
	categoria := strings.TrimSpace(r.FormValue("categoria"))
	tipo := strings.TrimSpace(r.FormValue("tipo"))
	naveOrigine := strings.TrimSpace(r.FormValue("nave_origine"))
	note := strings.TrimSpace(r.FormValue("note"))

	// Validazione
	if codice == "" || nome == "" {
		data.Error = "Codice e nome sono obbligatori"
		data.Data = map[string]interface{}{"FormData": formData}
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

	_, err = database.DB.Exec(`
		UPDATE prodotti SET codice = ?, nome = ?, descrizione = ?, categoria = ?, tipo = ?,
		       nave_origine = ?, unita_misura = ?, note = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, codice, nome, descrizione, categoria, tipo, naveOrigine, unitaMisura, note, id)

	if err != nil {
		data.Error = "Errore durante il salvataggio"
		data.Data = map[string]interface{}{"FormData": formData}
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
	
	tx, _ := database.DB.Begin()
	// Elimina prima i movimenti collegati
	tx.Exec("DELETE FROM movimenti_acquisto WHERE prodotto_id = ?", id)
	// Poi elimina il prodotto
	tx.Exec("DELETE FROM prodotti WHERE id = ?", id)
	tx.Commit()
	
	http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
}

// caricaMovimentiAcquisto recupera i movimenti acquisto per un prodotto
func caricaMovimentiAcquisto(prodottoID int64) []MovimentoAcquisto {
	var movimenti []MovimentoAcquisto
	rows, err := database.DB.Query(`
		SELECT m.id, m.prodotto_id, m.ddt_fattura_id, m.quantita, COALESCE(m.note, ''), m.created_at,
		       d.tipo, d.numero, d.data_documento, f.nome, COALESCE(f.is_amazon, 0)
		FROM movimenti_acquisto m
		LEFT JOIN ddt_fatture d ON m.ddt_fattura_id = d.id
		LEFT JOIN fornitori f ON d.fornitore_id = f.id
		WHERE m.prodotto_id = ?
		ORDER BY m.created_at DESC
	`, prodottoID)
	if err != nil {
		return movimenti
	}
	defer rows.Close()

	for rows.Next() {
		var m MovimentoAcquisto
		rows.Scan(&m.ID, &m.ProdottoID, &m.DDTFatturaID, &m.Quantita, &m.Note, &m.CreatedAt,
			&m.TipoDoc, &m.NumeroDoc, &m.DataDoc, &m.NomeFornitore, &m.IsAmazon)
		movimenti = append(movimenti, m)
	}
	return movimenti
}

// ============================================
// MOVIMENTI ACQUISTO (aggiunta/modifica/eliminazione)
// ============================================

// AggiungiMovimentoAcquisto aggiunge un nuovo acquisto a un prodotto esistente
func AggiungiMovimentoAcquisto(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metodo non consentito", http.StatusMethodNotAllowed)
		return
	}

	r.ParseForm()
	prodottoIDStr := r.FormValue("prodotto_id")
	ddtFatturaIDStr := r.FormValue("ddt_fattura_id")
	quantitaStr := r.FormValue("quantita")
	note := strings.TrimSpace(r.FormValue("note"))

	prodottoID, _ := strconv.ParseInt(prodottoIDStr, 10, 64)
	ddtFatturaID, _ := strconv.ParseInt(ddtFatturaIDStr, 10, 64)
	quantita, _ := strconv.Atoi(quantitaStr)

	if prodottoID == 0 || ddtFatturaID == 0 || quantita <= 0 {
		http.Redirect(w, r, "/magazzino/modifica/"+prodottoIDStr, http.StatusSeeOther)
		return
	}

	tx, _ := database.DB.Begin()

	// Inserisci movimento
	_, err := tx.Exec(`
		INSERT INTO movimenti_acquisto (prodotto_id, ddt_fattura_id, quantita, note)
		VALUES (?, ?, ?, ?)
	`, prodottoID, ddtFatturaID, quantita, note)
	if err != nil {
		tx.Rollback()
		http.Redirect(w, r, "/magazzino/modifica/"+prodottoIDStr, http.StatusSeeOther)
		return
	}

	// Aggiorna giacenza
	tx.Exec(`UPDATE prodotti SET giacenza = giacenza + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, quantita, prodottoID)

	tx.Commit()
	http.Redirect(w, r, "/magazzino/modifica/"+prodottoIDStr, http.StatusSeeOther)
}

// EliminaMovimentoAcquisto elimina un movimento e ricalcola la giacenza
func EliminaMovimentoAcquisto(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
		return
	}

	id, _ := strconv.ParseInt(pathParts[3], 10, 64)

	// Recupera info movimento
	var prodottoID int64
	var quantita int
	err := database.DB.QueryRow(`SELECT prodotto_id, quantita FROM movimenti_acquisto WHERE id = ?`, id).Scan(&prodottoID, &quantita)
	if err != nil {
		http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
		return
	}

	tx, _ := database.DB.Begin()

	// Elimina movimento
	tx.Exec(`DELETE FROM movimenti_acquisto WHERE id = ?`, id)

	// Sottrai dalla giacenza
	tx.Exec(`UPDATE prodotti SET giacenza = giacenza - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, quantita, prodottoID)

	tx.Commit()
	http.Redirect(w, r, "/magazzino/modifica/"+strconv.FormatInt(prodottoID, 10), http.StatusSeeOther)
}

// ============================================
// API
// ============================================

// APIDettaglioProdotto restituisce i dettagli di un prodotto
func APIDettaglioProdotto(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	idStr := r.URL.Query().Get("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	var p models.Prodotto
	var descrizione, naveOrigine, note sql.NullString

	err := database.DB.QueryRow(`
		SELECT id, codice, nome, descrizione, categoria, tipo, origine, nave_origine, giacenza, unita_misura, note
		FROM prodotti WHERE id = ?
	`, id).Scan(&p.ID, &p.Codice, &p.Nome, &descrizione, &p.Categoria, &p.Tipo, &p.Origine,
		&naveOrigine, &p.Giacenza, &p.UnitaMisura, &note)

	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "Prodotto non trovato"})
		return
	}

	if descrizione.Valid {
		p.Descrizione = descrizione.String
	}
	if naveOrigine.Valid {
		p.NaveOrigine = naveOrigine.String
	}
	if note.Valid {
		p.Note = note.String
	}

	// Aggiungi movimenti acquisto
	movimenti := caricaMovimentiAcquisto(id)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"prodotto":  p,
		"movimenti": movimenti,
	})
}

// ============================================
// VECCHIE FUNZIONI (mantenute per compatibilità DDT uscita)
// ============================================

// ListaMovimenti mostra lo storico movimenti di un prodotto (vecchio sistema)
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

	// Carica movimenti acquisto
	movimenti := caricaMovimentiAcquisto(prodottoID)

	type MovimentiData struct {
		Prodotto  models.Prodotto
		Movimenti []MovimentoAcquisto
	}

	data.Data = MovimentiData{
		Prodotto:  prodotto,
		Movimenti: movimenti,
	}
	renderTemplate(w, "movimenti_lista.html", data)
}

// NuovoMovimento - placeholder per compatibilità
func NuovoMovimento(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/magazzino", http.StatusSeeOther)
}
