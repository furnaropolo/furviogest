package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"furviogest/internal/auth"
	"furviogest/internal/database"
	"furviogest/internal/middleware"
)

// DDT struttura per DDT
type DDT struct {
	ID            int64
	Numero        string
	TipoDDT       string
	RapportoID    *int64
	NaveID        int64
	CompagniaID   int64
	PortoID       *int64
	Destinatario  string
	Indirizzo     string
	Vettore       string
	DataEmissione string
	Note          string
	// Campi join
	NomeNave      string
	NomeCompagnia string
	NomePorto     string
}

// RigaDDT struttura per riga DDT
type RigaDDT struct {
	ID          int64
	DDTID       int64
	ProdottoID  int64
	Quantita    float64
	Descrizione string
	// Campi join
	NomeProdotto string
	Unita        string
}

// ListaDDT mostra lista DDT
func ListaDDT(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	meseFilter := r.URL.Query().Get("mese")
	annoFilter := r.URL.Query().Get("anno")
	tipoFilter := r.URL.Query().Get("tipo")

	query := `
		SELECT d.id, d.numero, d.tipo_ddt, d.data_emissione,
		       COALESCE(n.nome, '') as nave, COALESCE(c.nome, '') as compagnia,
		       COALESCE(p.nome, '') as porto
		FROM ddt d
		LEFT JOIN navi n ON d.nave_id = n.id
		LEFT JOIN compagnie c ON d.compagnia_id = c.id
		LEFT JOIN porti p ON d.porto_id = p.id
		WHERE 1=1
	`

	var args []interface{}
	if meseFilter != "" && annoFilter != "" {
		query += " AND strftime('%m', d.data_emissione) = ? AND strftime('%Y', d.data_emissione) = ?"
		args = append(args, meseFilter, annoFilter)
	}
	if tipoFilter != "" {
		query += " AND d.tipo_ddt = ?"
		args = append(args, tipoFilter)
	}

	query += " ORDER BY d.data_emissione DESC, d.numero DESC LIMIT 200"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Errore caricamento DDT", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var ddts []DDT
	for rows.Next() {
		var d DDT
		err := rows.Scan(&d.ID, &d.Numero, &d.TipoDDT, &d.DataEmissione, &d.NomeNave, &d.NomeCompagnia, &d.NomePorto)
		if err != nil {
			continue
		}
		ddts = append(ddts, d)
	}

	pageData := NewPageData("DDT", r)
	pageData.Data = map[string]interface{}{
		"DDTs":       ddts,
		"MeseFilter": meseFilter,
		"AnnoFilter": annoFilter,
		"TipoFilter": tipoFilter,
	}

	renderTemplate(w, "ddt_lista.html", pageData)
}

// NuovoDDT gestisce creazione nuovo DDT
func NuovoDDT(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == "POST" {
		r.ParseForm()
		numero := r.FormValue("numero")
		tipoDdt := r.FormValue("tipo_ddt")
		naveID := r.FormValue("nave_id")
		compagniaID := r.FormValue("compagnia_id")
		portoID := r.FormValue("porto_id")
		destinatario := r.FormValue("destinatario")
		indirizzo := r.FormValue("indirizzo")
		vettore := r.FormValue("vettore")
		dataEmissione := r.FormValue("data_emissione")
		note := r.FormValue("note")

		var portoPtr *string
		if portoID != "" {
			portoPtr = &portoID
		}

		result, err := database.DB.Exec(`
			INSERT INTO ddt (numero, tipo_ddt, nave_id, compagnia_id, porto_id, destinatario, indirizzo, vettore, data_emissione, note)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, numero, tipoDdt, naveID, compagniaID, portoPtr, destinatario, indirizzo, vettore, dataEmissione, note)

		if err != nil {
			pageData := NewPageData("Nuovo DDT", r)
			pageData.Error = "Errore creazione DDT: " + err.Error()
			pageData.Data = getDDTFormData(nil)
			renderTemplate(w, "ddt_form.html", pageData)
			return
		}

		ddtID, _ := result.LastInsertId()
		http.Redirect(w, r, fmt.Sprintf("/ddt/dettaglio/%d", ddtID), http.StatusSeeOther)
		return
	}

	pageData := NewPageData("Nuovo DDT", r)
	pageData.Data = getDDTFormData(nil)
	renderTemplate(w, "ddt_form.html", pageData)
}

// ModificaDDT gestisce modifica DDT
func ModificaDDT(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/ddt/modifica/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/ddt", http.StatusSeeOther)
		return
	}

	if r.Method == "POST" {
		r.ParseForm()
		numero := r.FormValue("numero")
		tipoDdt := r.FormValue("tipo_ddt")
		naveID := r.FormValue("nave_id")
		compagniaID := r.FormValue("compagnia_id")
		portoID := r.FormValue("porto_id")
		destinatario := r.FormValue("destinatario")
		indirizzo := r.FormValue("indirizzo")
		vettore := r.FormValue("vettore")
		dataEmissione := r.FormValue("data_emissione")
		note := r.FormValue("note")

		var portoPtr *string
		if portoID != "" {
			portoPtr = &portoID
		}

		_, err = database.DB.Exec(`
			UPDATE ddt SET numero=?, tipo_ddt=?, nave_id=?, compagnia_id=?, porto_id=?,
			       destinatario=?, indirizzo=?, vettore=?, data_emissione=?, note=?
			WHERE id=?
		`, numero, tipoDdt, naveID, compagniaID, portoPtr, destinatario, indirizzo, vettore, dataEmissione, note, id)

		if err != nil {
			pageData := NewPageData("Modifica DDT", r)
			pageData.Error = "Errore modifica DDT: " + err.Error()
			ddt := getDDTByID(id)
			pageData.Data = getDDTFormData(ddt)
			renderTemplate(w, "ddt_form.html", pageData)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/ddt/dettaglio/%d", id), http.StatusSeeOther)
		return
	}

	ddt := getDDTByID(id)
	if ddt == nil {
		http.Redirect(w, r, "/ddt", http.StatusSeeOther)
		return
	}

	pageData := NewPageData("Modifica DDT", r)
	pageData.Data = getDDTFormData(ddt)
	renderTemplate(w, "ddt_form.html", pageData)
}

// DettaglioDDT mostra dettaglio DDT con righe
func DettaglioDDT(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/ddt/dettaglio/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/ddt", http.StatusSeeOther)
		return
	}

	ddt := getDDTByID(id)
	if ddt == nil {
		http.Redirect(w, r, "/ddt", http.StatusSeeOther)
		return
	}

	righe := getRigheDDT(id)
	prodotti, _ := getProdottiList()

	pageData := NewPageData("Dettaglio DDT", r)
	pageData.Data = map[string]interface{}{
		"DDT":      ddt,
		"Righe":    righe,
		"Prodotti": prodotti,
	}

	renderTemplate(w, "ddt_dettaglio.html", pageData)
}

// EliminaDDT elimina DDT
func EliminaDDT(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil || !session.IsTecnico() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/ddt/elimina/")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	database.DB.Exec("DELETE FROM ddt WHERE id = ?", id)
	http.Redirect(w, r, "/ddt", http.StatusSeeOther)
}

// AggiungiRigaDDT aggiunge riga a DDT
func AggiungiRigaDDT(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil || !session.IsTecnico() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method != "POST" {
		http.Redirect(w, r, "/ddt", http.StatusSeeOther)
		return
	}

	r.ParseForm()
	ddtID := r.FormValue("ddt_id")
	prodottoID := r.FormValue("prodotto_id")
	quantitaStr := r.FormValue("quantita")
	descrizione := r.FormValue("descrizione")

	quantita, _ := strconv.ParseFloat(quantitaStr, 64)
	if quantita <= 0 {
		quantita = 1
	}

	// Inserisce riga
	database.DB.Exec(`
		INSERT INTO righe_ddt (ddt_id, prodotto_id, quantita, descrizione)
		VALUES (?, ?, ?, ?)
	`, ddtID, prodottoID, quantita, descrizione)

	// Scarica dal magazzino
	database.DB.Exec(`
		UPDATE prodotti SET quantita = quantita - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, quantita, prodottoID)

	// Registra movimento
	database.DB.Exec(`
		INSERT INTO movimenti_magazzino (prodotto_id, tipo_movimento, quantita, motivo, data_movimento)
		VALUES (?, 'uscita', ?, 'DDT Uscita', ?)
	`, prodottoID, quantita, time.Now().Format("2006-01-02"))

	http.Redirect(w, r, "/ddt/dettaglio/"+ddtID, http.StatusSeeOther)
}

// RimuoviRigaDDT rimuove riga da DDT
func RimuoviRigaDDT(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil || !session.IsTecnico() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/ddt/riga/elimina/")
	parts := strings.Split(idStr, "/")
	if len(parts) < 2 {
		http.Redirect(w, r, "/ddt", http.StatusSeeOther)
		return
	}

	rigaID, _ := strconv.ParseInt(parts[0], 10, 64)
	ddtID := parts[1]

	// Recupera info riga per ripristinare magazzino
	var prodottoID int64
	var quantita float64
	database.DB.QueryRow("SELECT prodotto_id, quantita FROM righe_ddt WHERE id = ?", rigaID).Scan(&prodottoID, &quantita)

	// Elimina riga
	database.DB.Exec("DELETE FROM righe_ddt WHERE id = ?", rigaID)

	// Ripristina magazzino
	database.DB.Exec("UPDATE prodotti SET quantita = quantita + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", quantita, prodottoID)

	// Registra movimento
	database.DB.Exec(`
		INSERT INTO movimenti_magazzino (prodotto_id, tipo_movimento, quantita, motivo, data_movimento)
		VALUES (?, 'entrata', ?, 'Annullo riga DDT', ?)
	`, prodottoID, quantita, time.Now().Format("2006-01-02"))

	http.Redirect(w, r, "/ddt/dettaglio/"+ddtID, http.StatusSeeOther)
}

// GeneraNumDDT genera numero progressivo DDT
func GeneraNumDDT(w http.ResponseWriter, r *http.Request) {
	anno := time.Now().Year()
	
	var maxNum int
	database.DB.QueryRow(`
		SELECT COALESCE(MAX(CAST(SUBSTR(numero, 1, INSTR(numero, '/') - 1) AS INTEGER)), 0)
		FROM ddt WHERE numero LIKE '%/' || ?
	`, anno).Scan(&maxNum)

	numero := fmt.Sprintf("%d/%d", maxNum+1, anno)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"numero": "%s"}`, numero)))
}

// getDDTByID recupera DDT per ID
func getDDTByID(id int64) *DDT {
	var d DDT
	var portoID *int64
	err := database.DB.QueryRow(`
		SELECT d.id, d.numero, d.tipo_ddt, d.nave_id, d.compagnia_id, d.porto_id,
		       COALESCE(d.destinatario, ''), COALESCE(d.indirizzo, ''), COALESCE(d.vettore, ''),
		       d.data_emissione, COALESCE(d.note, ''),
		       COALESCE(n.nome, ''), COALESCE(c.nome, ''), COALESCE(p.nome, '')
		FROM ddt d
		LEFT JOIN navi n ON d.nave_id = n.id
		LEFT JOIN compagnie c ON d.compagnia_id = c.id
		LEFT JOIN porti p ON d.porto_id = p.id
		WHERE d.id = ?
	`, id).Scan(&d.ID, &d.Numero, &d.TipoDDT, &d.NaveID, &d.CompagniaID, &portoID,
		&d.Destinatario, &d.Indirizzo, &d.Vettore, &d.DataEmissione, &d.Note,
		&d.NomeNave, &d.NomeCompagnia, &d.NomePorto)

	if err != nil {
		return nil
	}
	d.PortoID = portoID
	return &d
}

// getRigheDDT recupera righe DDT
func getRigheDDT(ddtID int64) []RigaDDT {
	rows, err := database.DB.Query(`
		SELECT r.id, r.ddt_id, r.prodotto_id, r.quantita, COALESCE(r.descrizione, ''),
		       COALESCE(p.nome, ''), COALESCE(p.unita_misura, '')
		FROM righe_ddt r
		LEFT JOIN prodotti p ON r.prodotto_id = p.id
		WHERE r.ddt_id = ?
	`, ddtID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var righe []RigaDDT
	for rows.Next() {
		var r RigaDDT
		rows.Scan(&r.ID, &r.DDTID, &r.ProdottoID, &r.Quantita, &r.Descrizione, &r.NomeProdotto, &r.Unita)
		righe = append(righe, r)
	}
	return righe
}

// getDDTFormData recupera dati per form DDT
func getDDTFormData(ddt *DDT) map[string]interface{} {
	navi, _ := getNaviList()
	compagnie, _ := getCompagnieList()
	porti, _ := getPortiList()

	if ddt == nil {
		ddt = &DDT{
			DataEmissione: time.Now().Format("2006-01-02"),
			TipoDDT:       "intervento",
		}
	}

	return map[string]interface{}{
		"DDT":       ddt,
		"Navi":      navi,
		"Compagnie": compagnie,
		"Porti":     porti,
	}
}

// getCompagnieList recupera lista compagnie
var _ = auth.HashPassword
