package handlers

import (
	"database/sql"
	"encoding/json"
	"furviogest/internal/database"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DDTFattura rappresenta un documento di acquisto
type DDTFattura struct {
	ID            int64
	FornitoreID   int64
	Tipo          string // "ddt", "fattura", "ordine"
	Numero        string
	DataDocumento time.Time
	Note          string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	// Campi virtuali per join
	NomeFornitore string
	IsAmazon      bool
}

// FornitoreSelect per select con flag amazon
type FornitoreSelect struct {
	ID       int64
	Nome     string
	IsAmazon bool
}

// ListaDDTFatture mostra la lista dei DDT/Fatture registrati
func ListaDDTFatture(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Registro DDT/Fatture - FurvioGest", r)

	rows, err := database.DB.Query(`
		SELECT d.id, d.fornitore_id, d.tipo, d.numero, d.data_documento, d.note, d.created_at,
		       f.nome as nome_fornitore, COALESCE(f.is_amazon, 0) as is_amazon
		FROM ddt_fatture d
		LEFT JOIN fornitori f ON d.fornitore_id = f.id
		ORDER BY d.data_documento DESC, d.id DESC
	`)
	if err != nil {
		data.Error = "Errore nel caricamento dei documenti: " + err.Error()
		renderTemplate(w, "ddt_fatture_lista.html", data)
		return
	}
	defer rows.Close()

	var ddts []DDTFattura
	for rows.Next() {
		var d DDTFattura
		var note sql.NullString
		err := rows.Scan(&d.ID, &d.FornitoreID, &d.Tipo, &d.Numero, &d.DataDocumento, &note, &d.CreatedAt, &d.NomeFornitore, &d.IsAmazon)
		if err != nil {
			continue
		}
		if note.Valid {
			d.Note = note.String
		}
		ddts = append(ddts, d)
	}

	data.Data = ddts
	renderTemplate(w, "ddt_fatture_lista.html", data)
}

// NuovoDDTFattura gestisce la creazione di un nuovo DDT/Fattura
func NuovoDDTFattura(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Nuovo DDT/Fattura - FurvioGest", r)

	// Carica fornitori per select (con flag amazon)
	fornitori := caricaFornitoriConAmazon()

	data.Data = map[string]interface{}{
		"Fornitori": fornitori,
	}

	if r.Method == http.MethodGet {
		renderTemplate(w, "ddt_fatture_form.html", data)
		return
	}

	// POST - Salva DDT
	r.ParseForm()

	fornitoreIDStr := r.FormValue("fornitore_id")
	tipo := r.FormValue("tipo")
	numero := strings.TrimSpace(r.FormValue("numero"))
	dataDocStr := r.FormValue("data_documento")
	note := strings.TrimSpace(r.FormValue("note"))

	// Validazioni
	fornitoreID, err := strconv.ParseInt(fornitoreIDStr, 10, 64)
	if err != nil || fornitoreID == 0 {
		data.Error = "Seleziona un fornitore"
		renderTemplate(w, "ddt_fatture_form.html", data)
		return
	}

	if numero == "" {
		data.Error = "Il numero documento è obbligatorio"
		renderTemplate(w, "ddt_fatture_form.html", data)
		return
	}

	dataDoc, err := time.Parse("2006-01-02", dataDocStr)
	if err != nil {
		data.Error = "Data documento non valida"
		renderTemplate(w, "ddt_fatture_form.html", data)
		return
	}

	// Inserisci DDT
	_, err = database.DB.Exec(`
		INSERT INTO ddt_fatture (fornitore_id, tipo, numero, data_documento, note)
		VALUES (?, ?, ?, ?, ?)
	`, fornitoreID, tipo, numero, dataDoc, note)
	if err != nil {
		data.Error = "Errore salvataggio: " + err.Error()
		renderTemplate(w, "ddt_fatture_form.html", data)
		return
	}

	http.Redirect(w, r, "/ddt-fatture", http.StatusSeeOther)
}

// ModificaDDTFattura gestisce la modifica di un DDT/Fattura
func ModificaDDTFattura(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Modifica DDT/Fattura - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/ddt-fatture", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/ddt-fatture", http.StatusSeeOther)
		return
	}

	fornitori := caricaFornitoriConAmazon()

	if r.Method == http.MethodGet {
		var d DDTFattura
		var note sql.NullString
		err := database.DB.QueryRow(`
			SELECT id, fornitore_id, tipo, numero, data_documento, note
			FROM ddt_fatture WHERE id = ?
		`, id).Scan(&d.ID, &d.FornitoreID, &d.Tipo, &d.Numero, &d.DataDocumento, &note)
		if err != nil {
			http.Redirect(w, r, "/ddt-fatture", http.StatusSeeOther)
			return
		}
		if note.Valid {
			d.Note = note.String
		}

		data.Data = map[string]interface{}{
			"DDT":       d,
			"Fornitori": fornitori,
		}
		renderTemplate(w, "ddt_fatture_form.html", data)
		return
	}

	// POST - Aggiorna DDT
	r.ParseForm()

	fornitoreIDStr := r.FormValue("fornitore_id")
	tipo := r.FormValue("tipo")
	numero := strings.TrimSpace(r.FormValue("numero"))
	dataDocStr := r.FormValue("data_documento")
	note := strings.TrimSpace(r.FormValue("note"))

	fornitoreID, _ := strconv.ParseInt(fornitoreIDStr, 10, 64)
	dataDoc, _ := time.Parse("2006-01-02", dataDocStr)

	_, err = database.DB.Exec(`
		UPDATE ddt_fatture SET fornitore_id=?, tipo=?, numero=?, data_documento=?, note=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?
	`, fornitoreID, tipo, numero, dataDoc, note, id)
	if err != nil {
		data.Error = "Errore aggiornamento: " + err.Error()
		data.Data = map[string]interface{}{"Fornitori": fornitori}
		renderTemplate(w, "ddt_fatture_form.html", data)
		return
	}

	http.Redirect(w, r, "/ddt-fatture", http.StatusSeeOther)
}

// EliminaDDTFattura elimina un DDT e tutti i movimenti collegati
func EliminaDDTFattura(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/ddt-fatture", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/ddt-fatture", http.StatusSeeOther)
		return
	}

	tx, _ := database.DB.Begin()

	// Trova tutti i prodotti collegati a questo DDT tramite movimenti_acquisto
	rows, _ := tx.Query(`
		SELECT prodotto_id, quantita FROM movimenti_acquisto WHERE ddt_fattura_id = ?
	`, id)
	
	var prodottiDaVerificare []int64
	for rows.Next() {
		var prodID int64
		var qta int
		rows.Scan(&prodID, &qta)
		// Sottrai la quantità dalla giacenza
		tx.Exec(`UPDATE prodotti SET giacenza = giacenza - ? WHERE id = ?`, qta, prodID)
		prodottiDaVerificare = append(prodottiDaVerificare, prodID)
	}
	rows.Close()

	// Elimina movimenti_acquisto collegati
	tx.Exec(`DELETE FROM movimenti_acquisto WHERE ddt_fattura_id = ?`, id)

	// Verifica ed elimina prodotti con giacenza 0 e nessun altro movimento
	for _, prodID := range prodottiDaVerificare {
		var countMov int
		tx.QueryRow(`SELECT COUNT(*) FROM movimenti_acquisto WHERE prodotto_id = ?`, prodID).Scan(&countMov)
		if countMov == 0 {
			// Nessun altro movimento, elimina il prodotto
			tx.Exec(`DELETE FROM prodotti WHERE id = ? AND origine = 'nuovo'`, prodID)
		}
	}

	// Elimina DDT
	tx.Exec(`DELETE FROM ddt_fatture WHERE id = ?`, id)

	tx.Commit()
	http.Redirect(w, r, "/ddt-fatture", http.StatusSeeOther)
}

// APIInfoEliminazioneDDTFattura restituisce info su cosa verrà eliminato
func APIInfoEliminazioneDDTFattura(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	var info struct {
		NumeroDoc         string   `json:"numero_doc"`
		NomeFornitore     string   `json:"nome_fornitore"`
		NumMovimenti      int      `json:"num_movimenti"`
		ProdottiEliminati []string `json:"prodotti_eliminati"`
	}

	database.DB.QueryRow(`
		SELECT d.numero, f.nome 
		FROM ddt_fatture d 
		LEFT JOIN fornitori f ON d.fornitore_id = f.id 
		WHERE d.id = ?
	`, id).Scan(&info.NumeroDoc, &info.NomeFornitore)

	database.DB.QueryRow(`SELECT COUNT(*) FROM movimenti_acquisto WHERE ddt_fattura_id = ?`, id).Scan(&info.NumMovimenti)

	// Prodotti che verranno eliminati (quelli senza altri movimenti)
	rows, _ := database.DB.Query(`
		SELECT p.nome FROM movimenti_acquisto m
		JOIN prodotti p ON m.prodotto_id = p.id
		WHERE m.ddt_fattura_id = ? AND p.origine = 'nuovo'
		AND NOT EXISTS (SELECT 1 FROM movimenti_acquisto m2 WHERE m2.prodotto_id = m.prodotto_id AND m2.ddt_fattura_id != ?)
	`, id, id)
	defer rows.Close()
	for rows.Next() {
		var nome string
		rows.Scan(&nome)
		info.ProdottiEliminati = append(info.ProdottiEliminati, nome)
	}

	json.NewEncoder(w).Encode(info)
}

// APICercaDDTFatture cerca DDT per fornitore/numero/data (per autocomplete nel form prodotti)
func APICercaDDTFatture(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	fornitoreIDStr := r.URL.Query().Get("fornitore_id")
	cerca := r.URL.Query().Get("q")

	query := `
		SELECT d.id, d.tipo, d.numero, d.data_documento, f.nome, COALESCE(f.is_amazon, 0)
		FROM ddt_fatture d
		LEFT JOIN fornitori f ON d.fornitore_id = f.id
		WHERE 1=1
	`
	var args []interface{}

	if fornitoreIDStr != "" {
		fornitoreID, _ := strconv.ParseInt(fornitoreIDStr, 10, 64)
		if fornitoreID > 0 {
			query += " AND d.fornitore_id = ?"
			args = append(args, fornitoreID)
		}
	}

	if cerca != "" {
		query += " AND (d.numero LIKE ? OR f.nome LIKE ?)"
		args = append(args, "%"+cerca+"%", "%"+cerca+"%")
	}

	query += " ORDER BY d.data_documento DESC LIMIT 20"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	defer rows.Close()

	var risultati []map[string]interface{}
	for rows.Next() {
		var id int64
		var tipo, numero, nomeFornitore string
		var dataDoc time.Time
		var isAmazon bool
		rows.Scan(&id, &tipo, &numero, &dataDoc, &nomeFornitore, &isAmazon)
		
		label := tipo
		if isAmazon {
			label = "Ordine"
		}
		
		risultati = append(risultati, map[string]interface{}{
			"id":        id,
			"tipo":      tipo,
			"numero":    numero,
			"data":      dataDoc.Format("02/01/2006"),
			"fornitore": nomeFornitore,
			"label":     label + " n." + numero + " del " + dataDoc.Format("02/01/2006") + " - " + nomeFornitore,
		})
	}

	json.NewEncoder(w).Encode(risultati)
}

// Helper per caricare fornitori con flag amazon
func caricaFornitoriConAmazon() []FornitoreSelect {
	rows, err := database.DB.Query(`SELECT id, nome, COALESCE(is_amazon, 0) FROM fornitori ORDER BY is_amazon DESC, nome`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var fornitori []FornitoreSelect
	for rows.Next() {
		var f FornitoreSelect
		rows.Scan(&f.ID, &f.Nome, &f.IsAmazon)
		fornitori = append(fornitori, f)
	}
	return fornitori
}
