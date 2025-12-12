package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"furviogest/internal/database"
	"furviogest/internal/models"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ============================================
// DDT USCITA
// ============================================

func ListaDDTUscita(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("DDT Uscita - FurvioGest", r)

	// Filtri
	annoStr := r.URL.Query().Get("anno")
	clienteIDStr := r.URL.Query().Get("cliente_id")
	mostraAnnullati := r.URL.Query().Get("annullati") == "1"

	anno := time.Now().Year()
	if annoStr != "" {
		if a, err := strconv.Atoi(annoStr); err == nil {
			anno = a
		}
	}

	// Query base
	query := `
		SELECT d.id, d.numero, d.anno, d.data_documento, d.cliente_id, 
		       COALESCE(d.destinazione,''), d.causale, d.porto, d.aspetto_beni,
		       COALESCE(d.nr_colli, 0), COALESCE(d.peso,''), d.data_ora_trasporto,
		       d.incaricato_trasporto, COALESCE(d.note,''), d.annullato, d.created_at,
		       c.nome as nome_cliente
		FROM ddt_uscita d
		JOIN clienti c ON d.cliente_id = c.id
		WHERE d.anno = ?
	`
	args := []interface{}{anno}

	if clienteIDStr != "" {
		if cid, err := strconv.ParseInt(clienteIDStr, 10, 64); err == nil {
			query += " AND d.cliente_id = ?"
			args = append(args, cid)
		}
	}

	if !mostraAnnullati {
		query += " AND d.annullato = 0"
	}

	query += " ORDER BY d.numero DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		data.Error = "Errore nel recupero dei DDT: " + err.Error()
		renderTemplate(w, "ddt_uscita_lista.html", data)
		return
	}
	defer rows.Close()

	var ddtList []models.DDTUscita
	for rows.Next() {
		var d models.DDTUscita
		var dataOraTrasporto sql.NullTime
		err := rows.Scan(&d.ID, &d.Numero, &d.Anno, &d.DataDocumento, &d.ClienteID,
			&d.Destinazione, &d.Causale, &d.Porto, &d.AspettoBeni,
			&d.NrColli, &d.Peso, &dataOraTrasporto,
			&d.IncaricatoTrasporto, &d.Note, &d.Annullato, &d.CreatedAt,
			&d.NomeCliente)
		if err != nil {
			continue
		}
		if dataOraTrasporto.Valid {
			d.DataOraTrasporto = &dataOraTrasporto.Time
		}
		ddtList = append(ddtList, d)
	}

	// Recupera lista clienti per filtro
	clienti, _ := getClientiList()

	// Recupera anni disponibili
	anni := getAnniDDTUscita()

	data.Data = map[string]interface{}{
		"DDT":              ddtList,
		"Clienti":          clienti,
		"Anni":             anni,
		"AnnoSelezionato":  anno,
		"ClienteSelezionato": clienteIDStr,
		"MostraAnnullati":  mostraAnnullati,
	}
	renderTemplate(w, "ddt_uscita_lista.html", data)
}

func NuovoDDTUscita(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Nuovo DDT Uscita - FurvioGest", r)

	if r.Method == http.MethodGet {
		clienti, _ := getClientiList()
		
		// Genera prossimo numero DDT
		anno := time.Now().Year()
		var maxNum int
		database.DB.QueryRow("SELECT COALESCE(MAX(CAST(numero AS INTEGER)), 0) FROM ddt_uscita WHERE anno = ?", anno).Scan(&maxNum)
		prossimoNumero := fmt.Sprintf("%d", maxNum+1)

		data.Data = map[string]interface{}{
			"Clienti":        clienti,
			"ProssimoNumero": prossimoNumero,
			"Anno":           anno,
			"DataDocumento":  time.Now().Format("2006-01-02"),
			"DataOraTrasporto": time.Now().Format("2006-01-02T15:04"),
		}
		renderTemplate(w, "ddt_uscita_form.html", data)
		return
	}

	// POST - salva DDT
	r.ParseForm()
	
	numero := strings.TrimSpace(r.FormValue("numero"))
	annoStr := r.FormValue("anno")
	dataDocStr := r.FormValue("data_documento")
	clienteIDStr := r.FormValue("cliente_id")
	destinazione := strings.TrimSpace(r.FormValue("destinazione"))
	causale := strings.TrimSpace(r.FormValue("causale"))
	if causale == "" {
		causale = r.FormValue("causale_custom")
	}
	porto := r.FormValue("porto")
	aspettoBeni := strings.TrimSpace(r.FormValue("aspetto_beni"))
	if aspettoBeni == "" {
		aspettoBeni = r.FormValue("aspetto_beni_custom")
	}
	nrColliStr := r.FormValue("nr_colli")
	peso := strings.TrimSpace(r.FormValue("peso"))
	dataOraTrasportoStr := r.FormValue("data_ora_trasporto")
	incaricatoTrasporto := r.FormValue("incaricato_trasporto")
	note := strings.TrimSpace(r.FormValue("note"))

	// Validazione
	if numero == "" || clienteIDStr == "" || dataDocStr == "" {
		data.Error = "Numero, Cliente e Data Documento sono obbligatori"
		clienti, _ := getClientiList()
		data.Data = map[string]interface{}{"Clienti": clienti}
		renderTemplate(w, "ddt_uscita_form.html", data)
		return
	}

	anno, _ := strconv.Atoi(annoStr)
	clienteID, _ := strconv.ParseInt(clienteIDStr, 10, 64)
	dataDoc, _ := time.Parse("2006-01-02", dataDocStr)
	nrColli, _ := strconv.Atoi(nrColliStr)
	
	var dataOraTrasporto *time.Time
	if dataOraTrasportoStr != "" {
		if t, err := time.Parse("2006-01-02T15:04", dataOraTrasportoStr); err == nil {
			dataOraTrasporto = &t
		}
	}

	// Verifica numero univoco per anno
	var exists int
	database.DB.QueryRow("SELECT COUNT(*) FROM ddt_uscita WHERE numero = ? AND anno = ?", numero, anno).Scan(&exists)
	if exists > 0 {
		data.Error = "Esiste già un DDT con questo numero per l'anno selezionato"
		clienti, _ := getClientiList()
		data.Data = map[string]interface{}{"Clienti": clienti}
		renderTemplate(w, "ddt_uscita_form.html", data)
		return
	}

	// Inserisci DDT
	result, err := database.DB.Exec(`
		INSERT INTO ddt_uscita (numero, anno, data_documento, cliente_id, destinazione, causale, porto, aspetto_beni, nr_colli, peso, data_ora_trasporto, incaricato_trasporto, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, numero, anno, dataDoc, clienteID, destinazione, causale, porto, aspettoBeni, nrColli, peso, dataOraTrasporto, incaricatoTrasporto, note)

	if err != nil {
		data.Error = "Errore durante il salvataggio: " + err.Error()
		clienti, _ := getClientiList()
		data.Data = map[string]interface{}{"Clienti": clienti}
		renderTemplate(w, "ddt_uscita_form.html", data)
		return
	}

	ddtID, _ := result.LastInsertId()
	http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d", ddtID), http.StatusSeeOther)
}

func DettaglioDDTUscita(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Dettaglio DDT Uscita - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/ddt-uscita", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/ddt-uscita", http.StatusSeeOther)
		return
	}

	// Recupera DDT
	var d models.DDTUscita
	var dataOraTrasporto sql.NullTime
	err = database.DB.QueryRow(`
		SELECT d.id, d.numero, d.anno, d.data_documento, d.cliente_id, 
		       COALESCE(d.destinazione,''), d.causale, d.porto, d.aspetto_beni,
		       COALESCE(d.nr_colli, 0), COALESCE(d.peso,''), d.data_ora_trasporto,
		       d.incaricato_trasporto, COALESCE(d.note,''), d.annullato, d.created_at,
		       c.nome as nome_cliente
		FROM ddt_uscita d
		JOIN clienti c ON d.cliente_id = c.id
		WHERE d.id = ?
	`, id).Scan(&d.ID, &d.Numero, &d.Anno, &d.DataDocumento, &d.ClienteID,
		&d.Destinazione, &d.Causale, &d.Porto, &d.AspettoBeni,
		&d.NrColli, &d.Peso, &dataOraTrasporto,
		&d.IncaricatoTrasporto, &d.Note, &d.Annullato, &d.CreatedAt,
		&d.NomeCliente)

	if err == sql.ErrNoRows {
		http.Redirect(w, r, "/ddt-uscita", http.StatusSeeOther)
		return
	}
	if err != nil {
		data.Error = "Errore nel recupero del DDT"
		renderTemplate(w, "ddt_uscita_dettaglio.html", data)
		return
	}

	if dataOraTrasporto.Valid {
		d.DataOraTrasporto = &dataOraTrasporto.Time
	}

	// Recupera righe DDT
	righe, _ := getRigheDDTUscita(id)
	d.Righe = righe

	// Recupera cliente completo per dati destinatario
	var cliente models.Cliente
	database.DB.QueryRow(`
		SELECT id, nome, COALESCE(indirizzo,''), COALESCE(cap,''), COALESCE(citta,''), COALESCE(provincia,'')
		FROM clienti WHERE id = ?
	`, d.ClienteID).Scan(&cliente.ID, &cliente.Nome, &cliente.Indirizzo, &cliente.CAP, &cliente.Citta, &cliente.Provincia)

	data.Data = map[string]interface{}{
		"DDT":     d,
		"Cliente": cliente,
	}
	renderTemplate(w, "ddt_uscita_dettaglio.html", data)
}

func ModificaDDTUscita(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Modifica DDT Uscita - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/ddt-uscita", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/ddt-uscita", http.StatusSeeOther)
		return
	}

	// Verifica che il DDT non sia annullato
	var annullato bool
	database.DB.QueryRow("SELECT annullato FROM ddt_uscita WHERE id = ?", id).Scan(&annullato)
	if annullato {
		http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d", id), http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		var d models.DDTUscita
		var dataOraTrasporto sql.NullTime
		err = database.DB.QueryRow(`
			SELECT id, numero, anno, data_documento, cliente_id, 
			       COALESCE(destinazione,''), causale, porto, aspetto_beni,
			       COALESCE(nr_colli, 0), COALESCE(peso,''), data_ora_trasporto,
			       incaricato_trasporto, COALESCE(note,'')
			FROM ddt_uscita WHERE id = ?
		`, id).Scan(&d.ID, &d.Numero, &d.Anno, &d.DataDocumento, &d.ClienteID,
			&d.Destinazione, &d.Causale, &d.Porto, &d.AspettoBeni,
			&d.NrColli, &d.Peso, &dataOraTrasporto,
			&d.IncaricatoTrasporto, &d.Note)

		if err != nil {
			http.Redirect(w, r, "/ddt-uscita", http.StatusSeeOther)
			return
		}

		if dataOraTrasporto.Valid {
			d.DataOraTrasporto = &dataOraTrasporto.Time
		}

		clienti, _ := getClientiList()

		data.Data = map[string]interface{}{
			"DDT":      d,
			"Clienti":  clienti,
		}
		renderTemplate(w, "ddt_uscita_form.html", data)
		return
	}

	// POST - salva modifiche
	r.ParseForm()

	dataDocStr := r.FormValue("data_documento")
	clienteIDStr := r.FormValue("cliente_id")
	destinazione := strings.TrimSpace(r.FormValue("destinazione"))
	causale := strings.TrimSpace(r.FormValue("causale"))
	if causale == "" {
		causale = r.FormValue("causale_custom")
	}
	porto := r.FormValue("porto")
	aspettoBeni := strings.TrimSpace(r.FormValue("aspetto_beni"))
	if aspettoBeni == "" {
		aspettoBeni = r.FormValue("aspetto_beni_custom")
	}
	nrColliStr := r.FormValue("nr_colli")
	peso := strings.TrimSpace(r.FormValue("peso"))
	dataOraTrasportoStr := r.FormValue("data_ora_trasporto")
	incaricatoTrasporto := r.FormValue("incaricato_trasporto")
	note := strings.TrimSpace(r.FormValue("note"))

	clienteID, _ := strconv.ParseInt(clienteIDStr, 10, 64)
	dataDoc, _ := time.Parse("2006-01-02", dataDocStr)
	nrColli, _ := strconv.Atoi(nrColliStr)

	var dataOraTrasporto *time.Time
	if dataOraTrasportoStr != "" {
		if t, err := time.Parse("2006-01-02T15:04", dataOraTrasportoStr); err == nil {
			dataOraTrasporto = &t
		}
	}

	_, err = database.DB.Exec(`
		UPDATE ddt_uscita SET data_documento=?, cliente_id=?, destinazione=?, causale=?, porto=?, aspetto_beni=?, nr_colli=?, peso=?, data_ora_trasporto=?, incaricato_trasporto=?, note=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?
	`, dataDoc, clienteID, destinazione, causale, porto, aspettoBeni, nrColli, peso, dataOraTrasporto, incaricatoTrasporto, note, id)

	if err != nil {
		data.Error = "Errore durante il salvataggio: " + err.Error()
		renderTemplate(w, "ddt_uscita_form.html", data)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d", id), http.StatusSeeOther)
}

func AnnullaDDTUscita(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/ddt-uscita", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/ddt-uscita", http.StatusSeeOther)
		return
	}

	// Inizia transazione
	tx, err := database.DB.Begin()
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d?error=1", id), http.StatusSeeOther)
		return
	}

	// Ripristina giacenze per ogni riga
	rows, err := tx.Query("SELECT prodotto_id, quantita FROM righe_ddt_uscita WHERE ddt_uscita_id = ?", id)
	if err != nil {
		tx.Rollback()
		http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d?error=1", id), http.StatusSeeOther)
		return
	}

	for rows.Next() {
		var prodottoID int64
		var quantita float64
		if err := rows.Scan(&prodottoID, &quantita); err != nil {
			continue
		}
		// Ripristina giacenza (aggiungi)
		tx.Exec("UPDATE prodotti SET giacenza = giacenza + ? WHERE id = ?", quantita, prodottoID)
	}
	rows.Close()

	// Marca DDT come annullato
	_, err = tx.Exec("UPDATE ddt_uscita SET annullato = 1, data_annullamento = CURRENT_TIMESTAMP WHERE id = ?", id)
	if err != nil {
		tx.Rollback()
		http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d?error=1", id), http.StatusSeeOther)
		return
	}

	tx.Commit()
	http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d?annullato=1", id), http.StatusSeeOther)
}

// Aggiungi riga a DDT
func AggiungiRigaDDTUscita(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.ParseForm()
	ddtIDStr := r.FormValue("ddt_id")
	prodottoIDStr := r.FormValue("prodotto_id")
	quantitaStr := r.FormValue("quantita")

	ddtID, _ := strconv.ParseInt(ddtIDStr, 10, 64)
	prodottoID, _ := strconv.ParseInt(prodottoIDStr, 10, 64)
	quantita, _ := strconv.ParseFloat(quantitaStr, 64)

	if ddtID == 0 || prodottoID == 0 || quantita <= 0 {
		http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d?error=parametri", ddtID), http.StatusSeeOther)
		return
	}

	// Verifica DDT non annullato
	var annullato bool
	database.DB.QueryRow("SELECT annullato FROM ddt_uscita WHERE id = ?", ddtID).Scan(&annullato)
	if annullato {
		http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d", ddtID), http.StatusSeeOther)
		return
	}

	// Verifica giacenza disponibile
	var giacenza float64
	database.DB.QueryRow("SELECT giacenza FROM prodotti WHERE id = ?", prodottoID).Scan(&giacenza)
	if giacenza < quantita {
		http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d?error=giacenza", ddtID), http.StatusSeeOther)
		return
	}

	// Inizia transazione
	tx, err := database.DB.Begin()
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d?error=db", ddtID), http.StatusSeeOther)
		return
	}

	// Inserisci riga
	_, err = tx.Exec("INSERT INTO righe_ddt_uscita (ddt_uscita_id, prodotto_id, quantita) VALUES (?, ?, ?)", ddtID, prodottoID, quantita)
	if err != nil {
		tx.Rollback()
		http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d?error=insert", ddtID), http.StatusSeeOther)
		return
	}

	// Scala giacenza
	_, err = tx.Exec("UPDATE prodotti SET giacenza = giacenza - ? WHERE id = ?", quantita, prodottoID)
	if err != nil {
		tx.Rollback()
		http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d?error=giacenza", ddtID), http.StatusSeeOther)
		return
	}

	tx.Commit()
	http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d", ddtID), http.StatusSeeOther)
}

// Rimuovi riga da DDT
func RimuoviRigaDDTUscita(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		http.Redirect(w, r, "/ddt-uscita", http.StatusSeeOther)
		return
	}

	rigaID, err := strconv.ParseInt(pathParts[4], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/ddt-uscita", http.StatusSeeOther)
		return
	}

	// Recupera info riga
	var ddtID, prodottoID int64
	var quantita float64
	err = database.DB.QueryRow("SELECT ddt_uscita_id, prodotto_id, quantita FROM righe_ddt_uscita WHERE id = ?", rigaID).Scan(&ddtID, &prodottoID, &quantita)
	if err != nil {
		http.Redirect(w, r, "/ddt-uscita", http.StatusSeeOther)
		return
	}

	// Verifica DDT non annullato
	var annullato bool
	database.DB.QueryRow("SELECT annullato FROM ddt_uscita WHERE id = ?", ddtID).Scan(&annullato)
	if annullato {
		http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d", ddtID), http.StatusSeeOther)
		return
	}

	// Inizia transazione
	tx, err := database.DB.Begin()
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d?error=db", ddtID), http.StatusSeeOther)
		return
	}

	// Elimina riga
	_, err = tx.Exec("DELETE FROM righe_ddt_uscita WHERE id = ?", rigaID)
	if err != nil {
		tx.Rollback()
		http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d?error=delete", ddtID), http.StatusSeeOther)
		return
	}

	// Ripristina giacenza
	_, err = tx.Exec("UPDATE prodotti SET giacenza = giacenza + ? WHERE id = ?", quantita, prodottoID)
	if err != nil {
		tx.Rollback()
		http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d?error=giacenza", ddtID), http.StatusSeeOther)
		return
	}

	tx.Commit()
	http.Redirect(w, r, fmt.Sprintf("/ddt-uscita/dettaglio/%d", ddtID), http.StatusSeeOther)
}

// API per cercare prodotti
func APICercaProdottiDDT(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	query := r.URL.Query().Get("q")
	fornitoreID := r.URL.Query().Get("fornitore_id")

	if query == "" && fornitoreID == "" {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	sqlQuery := `
		SELECT p.id, p.codice, p.nome, p.giacenza, p.unita_misura, COALESCE(f.nome, '') as fornitore
		FROM prodotti p
		LEFT JOIN fornitori f ON p.fornitore_id = f.id
		WHERE p.giacenza > 0
	`
	args := []interface{}{}

	if query != "" {
		sqlQuery += " AND (p.codice LIKE ? OR p.nome LIKE ?)"
		searchTerm := "%" + query + "%"
		args = append(args, searchTerm, searchTerm)
	}

	if fornitoreID != "" {
		if fid, err := strconv.ParseInt(fornitoreID, 10, 64); err == nil {
			sqlQuery += " AND p.fornitore_id = ?"
			args = append(args, fid)
		}
	}

	sqlQuery += " ORDER BY p.nome LIMIT 50"

	rows, err := database.DB.Query(sqlQuery, args...)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type ProdottoRicerca struct {
		ID          int64   `json:"id"`
		Codice      string  `json:"codice"`
		Nome        string  `json:"nome"`
		Giacenza    float64 `json:"giacenza"`
		UnitaMisura string  `json:"unita_misura"`
		Fornitore   string  `json:"fornitore"`
	}

	var prodotti []ProdottoRicerca
	for rows.Next() {
		var p ProdottoRicerca
		if err := rows.Scan(&p.ID, &p.Codice, &p.Nome, &p.Giacenza, &p.UnitaMisura, &p.Fornitore); err != nil {
			continue
		}
		prodotti = append(prodotti, p)
	}

	json.NewEncoder(w).Encode(prodotti)
}

// Helper functions

func getClientiList() ([]models.Cliente, error) {
	rows, err := database.DB.Query("SELECT id, nome FROM clienti ORDER BY nome")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clienti []models.Cliente
	for rows.Next() {
		var c models.Cliente
		if err := rows.Scan(&c.ID, &c.Nome); err != nil {
			continue
		}
		clienti = append(clienti, c)
	}
	return clienti, nil
}

func getRigheDDTUscita(ddtID int64) ([]models.RigaDDTUscita, error) {
	rows, err := database.DB.Query(`
		SELECT r.id, r.ddt_uscita_id, r.prodotto_id, r.quantita, COALESCE(r.descrizione,''),
		       p.codice, p.nome, p.unita_misura
		FROM righe_ddt_uscita r
		JOIN prodotti p ON r.prodotto_id = p.id
		WHERE r.ddt_uscita_id = ?
		ORDER BY r.id
	`, ddtID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var righe []models.RigaDDTUscita
	for rows.Next() {
		var r models.RigaDDTUscita
		if err := rows.Scan(&r.ID, &r.DDTUscitaID, &r.ProdottoID, &r.Quantita, &r.Descrizione,
			&r.CodiceProdotto, &r.NomeProdotto, &r.UnitaMisura); err != nil {
			continue
		}
		righe = append(righe, r)
	}
	return righe, nil
}

func getAnniDDTUscita() []int {
	rows, _ := database.DB.Query("SELECT DISTINCT anno FROM ddt_uscita ORDER BY anno DESC")
	defer rows.Close()

	var anni []int
	for rows.Next() {
		var anno int
		if rows.Scan(&anno) == nil {
			anni = append(anni, anno)
		}
	}

	// Aggiungi anno corrente se non presente
	currentYear := time.Now().Year()
	found := false
	for _, a := range anni {
		if a == currentYear {
			found = true
			break
		}
	}
	if !found {
		anni = append([]int{currentYear}, anni...)
	}

	return anni
}

// PDFDDTUscita genera la pagina stampabile del DDT
func PDFDDTUscita(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/ddt-uscita", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/ddt-uscita", http.StatusSeeOther)
		return
	}

	// Recupera DDT
	var d models.DDTUscita
	var dataOraTrasporto sql.NullTime
	err = database.DB.QueryRow(`
		SELECT d.id, d.numero, d.anno, d.data_documento, d.cliente_id, 
		       COALESCE(d.destinazione,''), d.causale, d.porto, d.aspetto_beni,
		       COALESCE(d.nr_colli, 0), COALESCE(d.peso,''), d.data_ora_trasporto,
		       d.incaricato_trasporto, COALESCE(d.note,''), d.annullato,
		       c.nome as nome_cliente
		FROM ddt_uscita d
		JOIN clienti c ON d.cliente_id = c.id
		WHERE d.id = ?
	`, id).Scan(&d.ID, &d.Numero, &d.Anno, &d.DataDocumento, &d.ClienteID,
		&d.Destinazione, &d.Causale, &d.Porto, &d.AspettoBeni,
		&d.NrColli, &d.Peso, &dataOraTrasporto,
		&d.IncaricatoTrasporto, &d.Note, &d.Annullato,
		&d.NomeCliente)

	if err != nil {
		http.Redirect(w, r, "/ddt-uscita", http.StatusSeeOther)
		return
	}

	if dataOraTrasporto.Valid {
		d.DataOraTrasporto = &dataOraTrasporto.Time
	}

	// Recupera cliente completo
	var cliente models.Cliente
	database.DB.QueryRow(`
		SELECT id, nome, COALESCE(indirizzo,''), COALESCE(cap,''), COALESCE(citta,''), 
		       COALESCE(provincia,''), COALESCE(nazione,'Italia')
		FROM clienti WHERE id = ?
	`, d.ClienteID).Scan(&cliente.ID, &cliente.Nome, &cliente.Indirizzo, &cliente.CAP, 
		&cliente.Citta, &cliente.Provincia, &cliente.Nazione)

	// Recupera righe DDT
	righe, _ := getRigheDDTUscita(id)
	d.Righe = righe

	// Recupera impostazioni azienda
	var azienda models.ImpostazioniAzienda
	database.DB.QueryRow(`
		SELECT COALESCE(ragione_sociale,''), COALESCE(partita_iva,''), COALESCE(indirizzo,''),
		       COALESCE(cap,''), COALESCE(citta,''), COALESCE(provincia,''),
		       COALESCE(telefono,''), COALESCE(email,''), COALESCE(pec,''),
		       COALESCE(sito_web,''), COALESCE(logo_path,'')
		FROM impostazioni_azienda WHERE id = 1
	`).Scan(&azienda.RagioneSociale, &azienda.PartitaIVA, &azienda.Indirizzo,
		&azienda.CAP, &azienda.Citta, &azienda.Provincia,
		&azienda.Telefono, &azienda.Email, &azienda.PEC,
		&azienda.SitoWeb, &azienda.LogoPath)

	data := map[string]interface{}{
		"DDT":     d,
		"Cliente": cliente,
		"Azienda": azienda,
	}

	renderTemplate(w, "ddt_uscita_pdf.html", data)
}
