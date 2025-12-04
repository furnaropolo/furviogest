package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"furviogest/internal/database"
	"furviogest/internal/middleware"
)

// TecnicoInfo contiene informazioni base sul tecnico
type TecnicoInfo struct {
	ID      int
	Nome    string
	Cognome string
}

// DashboardAmministrazione mostra la dashboard per l'ufficio contabilità
func DashboardAmministrazione(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Statistiche per la dashboard
	var totProdotti, totRapporti, totNoteSpese, totTrasferte, totDDT int
	database.DB.QueryRow("SELECT COUNT(*) FROM prodotti WHERE deleted_at IS NULL").Scan(&totProdotti)
	database.DB.QueryRow("SELECT COUNT(*) FROM rapporti WHERE deleted_at IS NULL").Scan(&totRapporti)
	database.DB.QueryRow("SELECT COUNT(*) FROM note_spese WHERE deleted_at IS NULL").Scan(&totNoteSpese)
	database.DB.QueryRow("SELECT COUNT(*) FROM trasferte WHERE deleted_at IS NULL").Scan(&totTrasferte)
	database.DB.QueryRow("SELECT COUNT(*) FROM ddt WHERE deleted_at IS NULL").Scan(&totDDT)

	// Lista tecnici per filtri
	tecnici, _ := getTecniciList()

	pageData := NewPageData("Dashboard Amministrazione", r)
	pageData.Data = map[string]interface{}{
		"TotProdotti":  totProdotti,
		"TotRapporti":  totRapporti,
		"TotNoteSpese": totNoteSpese,
		"TotTrasferte": totTrasferte,
		"TotDDT":       totDDT,
		"Tecnici":      tecnici,
	}

	renderTemplate(w, "amministrazione_dashboard.html", pageData)
}

// GiacenzaMagazzino mostra la giacenza di magazzino (solo lettura)
func GiacenzaMagazzino(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Query prodotti con giacenza
	rows, err := database.DB.Query(`
		SELECT p.id, p.codice, p.nome, p.descrizione, p.categoria, p.unita_misura,
		       p.quantita, p.scorta_minima, p.prezzo_acquisto, p.fornitore_id,
		       COALESCE(f.nome, '') as fornitore_nome
		FROM prodotti p
		LEFT JOIN fornitori f ON p.fornitore_id = f.id
		WHERE p.deleted_at IS NULL
		ORDER BY p.categoria, p.nome
	`)
	if err != nil {
		http.Error(w, "Errore caricamento prodotti", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var prodotti []map[string]interface{}
	for rows.Next() {
		var id int
		var codice, nome, descrizione, categoria, unitaMisura, fornitoreNome string
		var quantita float64
		var scortaMinima float64
		var prezzoAcquisto float64
		var fornitoreID *int

		err := rows.Scan(&id, &codice, &nome, &descrizione, &categoria, &unitaMisura,
			&quantita, &scortaMinima, &prezzoAcquisto, &fornitoreID, &fornitoreNome)
		if err != nil {
			continue
		}

		prodotti = append(prodotti, map[string]interface{}{
			"ID":            id,
			"Codice":        codice,
			"Nome":          nome,
			"Descrizione":   descrizione,
			"Categoria":     categoria,
			"UnitaMisura":   unitaMisura,
			"Quantita":      quantita,
			"ScortaMinima":  scortaMinima,
			"PrezzoAcquisto": prezzoAcquisto,
			"FornitoreNome": fornitoreNome,
			"SottoScorta":   quantita < scortaMinima,
		})
	}

	pageData := NewPageData("Giacenza Magazzino", r)
	pageData.Data = map[string]interface{}{
		"Prodotti": prodotti,
	}

	renderTemplate(w, "amministrazione_magazzino.html", pageData)
}

// ExportMagazzinoCSV esporta la giacenza in formato CSV
func ExportMagazzinoCSV(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Error(w, "Non autorizzato", http.StatusUnauthorized)
		return
	}

	rows, err := database.DB.Query(`
		SELECT p.codice, p.nome, p.descrizione, p.categoria, p.unita_misura,
		       p.quantita, p.scorta_minima, p.prezzo_acquisto,
		       COALESCE(f.nome, '') as fornitore
		FROM prodotti p
		LEFT JOIN fornitori f ON p.fornitore_id = f.id
		WHERE p.deleted_at IS NULL
		ORDER BY p.categoria, p.nome
	`)
	if err != nil {
		http.Error(w, "Errore esportazione", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Imposta header per download CSV
	filename := fmt.Sprintf("giacenza_magazzino_%s.csv", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// BOM per Excel
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	writer.Comma = ';'

	// Header
	writer.Write([]string{"Codice", "Nome", "Descrizione", "Categoria", "Unità Misura", "Quantità", "Scorta Minima", "Prezzo Acquisto", "Fornitore"})

	for rows.Next() {
		var codice, nome, descrizione, categoria, unitaMisura, fornitore string
		var quantita, scortaMinima, prezzoAcquisto float64

		rows.Scan(&codice, &nome, &descrizione, &categoria, &unitaMisura, &quantita, &scortaMinima, &prezzoAcquisto, &fornitore)

		writer.Write([]string{
			codice,
			nome,
			descrizione,
			categoria,
			unitaMisura,
			fmt.Sprintf("%.2f", quantita),
			fmt.Sprintf("%.2f", scortaMinima),
			fmt.Sprintf("%.2f €", prezzoAcquisto),
			fornitore,
		})
	}

	writer.Flush()
}

// ListaRapportiAmministrazione mostra i rapporti intervento (solo lettura)
func ListaRapportiAmministrazione(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Filtri
	tecnicoFilter := r.URL.Query().Get("tecnico")
	meseFilter := r.URL.Query().Get("mese")
	annoFilter := r.URL.Query().Get("anno")

	query := `
		SELECT r.id, r.numero, r.data_intervento, r.descrizione, r.stato,
		       COALESCE(n.nome, '') as nave, COALESCE(c.nome, '') as compagnia,
		       GROUP_CONCAT(DISTINCT u.nome || ' ' || u.cognome) as tecnici
		FROM rapporti r
		LEFT JOIN navi n ON r.nave_id = n.id
		LEFT JOIN compagnie c ON n.compagnia_id = c.id
		LEFT JOIN rapporti_tecnici rt ON r.id = rt.rapporto_id
		LEFT JOIN utenti u ON rt.tecnico_id = u.id
		WHERE r.deleted_at IS NULL
	`

	var args []interface{}
	if tecnicoFilter != "" {
		query += " AND rt.tecnico_id = ?"
		args = append(args, tecnicoFilter)
	}
	if meseFilter != "" && annoFilter != "" {
		query += " AND strftime('%m', r.data_intervento) = ? AND strftime('%Y', r.data_intervento) = ?"
		args = append(args, meseFilter, annoFilter)
	}

	query += " GROUP BY r.id ORDER BY r.data_intervento DESC LIMIT 100"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Errore caricamento rapporti", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var rapporti []map[string]interface{}
	for rows.Next() {
		var id int
		var numero, descrizione, stato, nave, compagnia string
		var tecnici *string
		var dataIntervento string

		rows.Scan(&id, &numero, &dataIntervento, &descrizione, &stato, &nave, &compagnia, &tecnici)

		tecniciStr := ""
		if tecnici != nil {
			tecniciStr = *tecnici
		}

		rapporti = append(rapporti, map[string]interface{}{
			"ID":             id,
			"Numero":         numero,
			"DataIntervento": dataIntervento,
			"Descrizione":    descrizione,
			"Stato":          stato,
			"Nave":           nave,
			"Compagnia":      compagnia,
			"Tecnici":        tecniciStr,
		})
	}

	tecniciList, _ := getTecniciList()

	pageData := NewPageData("Rapporti Intervento", r)
	pageData.Data = map[string]interface{}{
		"Rapporti":       rapporti,
		"Tecnici":        tecniciList,
		"TecnicoFilter":  tecnicoFilter,
		"MeseFilter":     meseFilter,
		"AnnoFilter":     annoFilter,
	}

	renderTemplate(w, "amministrazione_rapporti.html", pageData)
}

// NoteSpeseAmministrazione mostra le note spese per tecnico
func NoteSpeseAmministrazione(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	tecnicoFilter := r.URL.Query().Get("tecnico")
	meseFilter := r.URL.Query().Get("mese")
	annoFilter := r.URL.Query().Get("anno")

	query := `
		SELECT ns.id, ns.data_spesa, ns.tipo_spesa, ns.descrizione, ns.importo,
		       u.nome || ' ' || u.cognome as tecnico,
		       COALESCE(tr.destinazione, '') as trasferta
		FROM note_spese ns
		JOIN utenti u ON ns.tecnico_id = u.id
		LEFT JOIN trasferte tr ON ns.trasferta_id = tr.id
		WHERE ns.deleted_at IS NULL
	`

	var args []interface{}
	if tecnicoFilter != "" {
		query += " AND ns.tecnico_id = ?"
		args = append(args, tecnicoFilter)
	}
	if meseFilter != "" && annoFilter != "" {
		query += " AND strftime('%m', ns.data_spesa) = ? AND strftime('%Y', ns.data_spesa) = ?"
		args = append(args, meseFilter, annoFilter)
	}

	query += " ORDER BY ns.data_spesa DESC LIMIT 200"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Errore caricamento note spese", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var noteSpese []map[string]interface{}
	var totale float64

	for rows.Next() {
		var id int
		var dataSpesa, tipoSpesa, descrizione, tecnico, trasferta string
		var importo float64

		rows.Scan(&id, &dataSpesa, &tipoSpesa, &descrizione, &importo, &tecnico, &trasferta)

		noteSpese = append(noteSpese, map[string]interface{}{
			"ID":         id,
			"DataSpesa":  dataSpesa,
			"TipoSpesa":  tipoSpesa,
			"Descrizione": descrizione,
			"Importo":    importo,
			"Tecnico":    tecnico,
			"Trasferta":  trasferta,
		})
		totale += importo
	}

	tecnici, _ := getTecniciList()

	pageData := NewPageData("Note Spese", r)
	pageData.Data = map[string]interface{}{
		"NoteSpese":     noteSpese,
		"Totale":        totale,
		"Tecnici":       tecnici,
		"TecnicoFilter": tecnicoFilter,
		"MeseFilter":    meseFilter,
		"AnnoFilter":    annoFilter,
	}

	renderTemplate(w, "amministrazione_note_spese.html", pageData)
}

// ExportNoteSpeseCSV esporta le note spese in CSV
func ExportNoteSpeseCSV(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Error(w, "Non autorizzato", http.StatusUnauthorized)
		return
	}

	tecnicoFilter := r.URL.Query().Get("tecnico")
	meseFilter := r.URL.Query().Get("mese")
	annoFilter := r.URL.Query().Get("anno")

	query := `
		SELECT ns.data_spesa, u.nome || ' ' || u.cognome as tecnico,
		       ns.tipo_spesa, ns.descrizione, ns.importo,
		       COALESCE(tr.destinazione, '') as trasferta
		FROM note_spese ns
		JOIN utenti u ON ns.tecnico_id = u.id
		LEFT JOIN trasferte tr ON ns.trasferta_id = tr.id
		WHERE ns.deleted_at IS NULL
	`

	var args []interface{}
	if tecnicoFilter != "" {
		query += " AND ns.tecnico_id = ?"
		args = append(args, tecnicoFilter)
	}
	if meseFilter != "" && annoFilter != "" {
		query += " AND strftime('%m', ns.data_spesa) = ? AND strftime('%Y', ns.data_spesa) = ?"
		args = append(args, meseFilter, annoFilter)
	}
	query += " ORDER BY u.cognome, ns.data_spesa"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Errore esportazione", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	filename := fmt.Sprintf("note_spese_%s.csv", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	writer.Comma = ';'
	writer.Write([]string{"Data", "Tecnico", "Tipo Spesa", "Descrizione", "Importo", "Trasferta"})

	var totale float64
	for rows.Next() {
		var dataSpesa, tecnico, tipoSpesa, descrizione, trasferta string
		var importo float64
		rows.Scan(&dataSpesa, &tecnico, &tipoSpesa, &descrizione, &importo, &trasferta)
		totale += importo

		writer.Write([]string{
			dataSpesa, tecnico, tipoSpesa, descrizione,
			fmt.Sprintf("%.2f €", importo), trasferta,
		})
	}

	writer.Write([]string{"", "", "", "TOTALE", fmt.Sprintf("%.2f €", totale), ""})
	writer.Flush()
}

// RiepilogoTrasferteAmministrazione mostra il riepilogo trasferte
func RiepilogoTrasferteAmministrazione(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	tecnicoFilter := r.URL.Query().Get("tecnico")
	meseFilter := r.URL.Query().Get("mese")
	annoFilter := r.URL.Query().Get("anno")

	query := `
		SELECT tr.id, tr.data_partenza, tr.data_rientro, tr.destinazione,
		       tr.motivo, tr.stato, COALESCE(tr.km_totali, 0), COALESCE(tr.indennita_totale, 0),
		       u.nome || ' ' || u.cognome as tecnico,
		       COALESCE(a.targa, '') as automezzo
		FROM trasferte tr
		JOIN utenti u ON tr.tecnico_id = u.id
		LEFT JOIN automezzi a ON tr.automezzo_id = a.id
		WHERE tr.deleted_at IS NULL
	`

	var args []interface{}
	if tecnicoFilter != "" {
		query += " AND tr.tecnico_id = ?"
		args = append(args, tecnicoFilter)
	}
	if meseFilter != "" && annoFilter != "" {
		query += " AND strftime('%m', tr.data_partenza) = ? AND strftime('%Y', tr.data_partenza) = ?"
		args = append(args, meseFilter, annoFilter)
	}

	query += " ORDER BY tr.data_partenza DESC LIMIT 100"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Errore caricamento trasferte: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var trasferte []map[string]interface{}
	var totKm float64
	var totIndennita float64

	for rows.Next() {
		var id int
		var dataPartenza, dataRientro, destinazione, motivo, stato, tecnico, automezzo string
		var kmTotali, indennitaTotale float64

		err := rows.Scan(&id, &dataPartenza, &dataRientro, &destinazione, &motivo, &stato,
			&kmTotali, &indennitaTotale, &tecnico, &automezzo)
		if err != nil {
			continue
		}

		trasferte = append(trasferte, map[string]interface{}{
			"ID":              id,
			"DataPartenza":    dataPartenza,
			"DataRientro":     dataRientro,
			"Destinazione":    destinazione,
			"Motivo":          motivo,
			"Stato":           stato,
			"KmTotali":        kmTotali,
			"IndennitaTotale": indennitaTotale,
			"Tecnico":         tecnico,
			"Automezzo":       automezzo,
		})
		totKm += kmTotali
		totIndennita += indennitaTotale
	}

	tecnici, _ := getTecniciList()

	pageData := NewPageData("Riepilogo Trasferte", r)
	pageData.Data = map[string]interface{}{
		"Trasferte":      trasferte,
		"TotKm":          totKm,
		"TotIndennita":   totIndennita,
		"Tecnici":        tecnici,
		"TecnicoFilter":  tecnicoFilter,
		"MeseFilter":     meseFilter,
		"AnnoFilter":     annoFilter,
	}

	renderTemplate(w, "amministrazione_trasferte.html", pageData)
}

// ExportTrasferteCSV esporta le trasferte in CSV
func ExportTrasferteCSV(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Error(w, "Non autorizzato", http.StatusUnauthorized)
		return
	}

	tecnicoFilter := r.URL.Query().Get("tecnico")
	meseFilter := r.URL.Query().Get("mese")
	annoFilter := r.URL.Query().Get("anno")

	query := `
		SELECT tr.data_partenza, tr.data_rientro, u.nome || ' ' || u.cognome as tecnico,
		       tr.destinazione, tr.motivo, tr.stato, COALESCE(tr.km_totali, 0), COALESCE(tr.indennita_totale, 0),
		       COALESCE(a.targa, '') as automezzo
		FROM trasferte tr
		JOIN utenti u ON tr.tecnico_id = u.id
		LEFT JOIN automezzi a ON tr.automezzo_id = a.id
		WHERE tr.deleted_at IS NULL
	`

	var args []interface{}
	if tecnicoFilter != "" {
		query += " AND tr.tecnico_id = ?"
		args = append(args, tecnicoFilter)
	}
	if meseFilter != "" && annoFilter != "" {
		query += " AND strftime('%m', tr.data_partenza) = ? AND strftime('%Y', tr.data_partenza) = ?"
		args = append(args, meseFilter, annoFilter)
	}
	query += " ORDER BY u.cognome, tr.data_partenza"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Errore esportazione", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	filename := fmt.Sprintf("trasferte_%s.csv", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	writer.Comma = ';'
	writer.Write([]string{"Data Partenza", "Data Rientro", "Tecnico", "Destinazione", "Motivo", "Stato", "Km Totali", "Indennità", "Automezzo"})

	var totKm, totIndennita float64
	for rows.Next() {
		var dataPartenza, dataRientro, tecnico, destinazione, motivo, stato, automezzo string
		var kmTotali, indennitaTotale float64
		rows.Scan(&dataPartenza, &dataRientro, &tecnico, &destinazione, &motivo, &stato, &kmTotali, &indennitaTotale, &automezzo)
		totKm += kmTotali
		totIndennita += indennitaTotale

		writer.Write([]string{
			dataPartenza, dataRientro, tecnico, destinazione, motivo, stato,
			fmt.Sprintf("%.0f", kmTotali),
			fmt.Sprintf("%.2f €", indennitaTotale),
			automezzo,
		})
	}

	writer.Write([]string{"", "", "", "", "", "TOTALE", fmt.Sprintf("%.0f km", totKm), fmt.Sprintf("%.2f €", totIndennita), ""})
	writer.Flush()
}

// Helper per ottenere lista tecnici (utenti con ruolo tecnico)
func getTecniciList() ([]TecnicoInfo, error) {
	rows, err := database.DB.Query(`
		SELECT id, nome, cognome FROM utenti WHERE ruolo = 'tecnico' AND deleted_at IS NULL ORDER BY cognome, nome
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tecnici []TecnicoInfo
	for rows.Next() {
		var t TecnicoInfo
		rows.Scan(&t.ID, &t.Nome, &t.Cognome)
		tecnici = append(tecnici, t)
	}
	return tecnici, nil
}

// RiepilogoMensile genera il riepilogo mensile per un tecnico
func RiepilogoMensile(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	tecnicoID := r.URL.Query().Get("tecnico")
	mese := r.URL.Query().Get("mese")
	anno := r.URL.Query().Get("anno")

	if tecnicoID == "" || mese == "" || anno == "" {
		tecnici, _ := getTecniciList()
		pageData := NewPageData("Riepilogo Mensile", r)
		pageData.Data = map[string]interface{}{
			"Tecnici": tecnici,
		}
		renderTemplate(w, "amministrazione_riepilogo_form.html", pageData)
		return
	}

	// Dati tecnico
	var tecnico TecnicoInfo
	database.DB.QueryRow("SELECT id, nome, cognome FROM utenti WHERE id = ?", tecnicoID).Scan(
		&tecnico.ID, &tecnico.Nome, &tecnico.Cognome)

	// Trasferte del mese
	trasferteRows, _ := database.DB.Query(`
		SELECT tr.data_partenza, tr.data_rientro, tr.destinazione, COALESCE(tr.km_totali, 0), COALESCE(tr.indennita_totale, 0)
		FROM trasferte tr
		WHERE tr.tecnico_id = ? AND tr.deleted_at IS NULL
		AND strftime('%m', tr.data_partenza) = ? AND strftime('%Y', tr.data_partenza) = ?
		ORDER BY tr.data_partenza
	`, tecnicoID, mese, anno)
	defer trasferteRows.Close()

	var trasferte []map[string]interface{}
	var totKm, totIndennita float64
	for trasferteRows.Next() {
		var dataP, dataR, dest string
		var km, ind float64
		trasferteRows.Scan(&dataP, &dataR, &dest, &km, &ind)
		trasferte = append(trasferte, map[string]interface{}{
			"DataPartenza": dataP, "DataRientro": dataR, "Destinazione": dest,
			"Km": km, "Indennita": ind,
		})
		totKm += km
		totIndennita += ind
	}

	// Note spese del mese
	speseRows, _ := database.DB.Query(`
		SELECT data_spesa, tipo_spesa, descrizione, importo
		FROM note_spese
		WHERE tecnico_id = ? AND deleted_at IS NULL
		AND strftime('%m', data_spesa) = ? AND strftime('%Y', data_spesa) = ?
		ORDER BY data_spesa
	`, tecnicoID, mese, anno)
	defer speseRows.Close()

	var spese []map[string]interface{}
	var totSpese float64
	for speseRows.Next() {
		var data, tipo, desc string
		var importo float64
		speseRows.Scan(&data, &tipo, &desc, &importo)
		spese = append(spese, map[string]interface{}{
			"Data": data, "Tipo": tipo, "Descrizione": desc, "Importo": importo,
		})
		totSpese += importo
	}

	// Nome mese in italiano
	mesiItaliani := []string{"", "Gennaio", "Febbraio", "Marzo", "Aprile", "Maggio", "Giugno",
		"Luglio", "Agosto", "Settembre", "Ottobre", "Novembre", "Dicembre"}
	meseInt, _ := strconv.Atoi(mese)
	nomeMese := ""
	if meseInt >= 1 && meseInt <= 12 {
		nomeMese = mesiItaliani[meseInt]
	}

	pageData := NewPageData("Riepilogo Mensile", r)
	pageData.Data = map[string]interface{}{
		"Tecnico":      tecnico,
		"Mese":         mese,
		"Anno":         anno,
		"NomeMese":     nomeMese,
		"Trasferte":    trasferte,
		"TotKm":        totKm,
		"TotIndennita": totIndennita,
		"Spese":        spese,
		"TotSpese":     totSpese,
		"Totale":       totIndennita + totSpese,
	}

	renderTemplate(w, "amministrazione_riepilogo.html", pageData)
}

// DDTAmministrazione mostra i DDT di uscita merce
func DDTAmministrazione(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	meseFilter := r.URL.Query().Get("mese")
	annoFilter := r.URL.Query().Get("anno")

	query := `
		SELECT d.id, d.numero, d.tipo_ddt, d.data_emissione,
		       COALESCE(n.nome, '') as nave, COALESCE(c.nome, '') as compagnia,
		       COALESCE(p.nome, '') as porto, COALESCE(d.destinatario, ''),
		       COALESCE(d.vettore, ''), COALESCE(d.note, '')
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

	query += " ORDER BY d.data_emissione DESC, d.numero DESC LIMIT 200"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Errore caricamento DDT", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var ddts []map[string]interface{}
	for rows.Next() {
		var id int
		var numero, tipoDdt, dataEmissione, nave, compagnia, porto, destinatario, vettore, note string

		rows.Scan(&id, &numero, &tipoDdt, &dataEmissione, &nave, &compagnia, &porto, &destinatario, &vettore, &note)

		ddts = append(ddts, map[string]interface{}{
			"ID":            id,
			"Numero":        numero,
			"TipoDDT":       tipoDdt,
			"DataEmissione": dataEmissione,
			"Nave":          nave,
			"Compagnia":     compagnia,
			"Porto":         porto,
			"Destinatario":  destinatario,
			"Vettore":       vettore,
			"Note":          note,
		})
	}

	pageData := NewPageData("DDT Uscita Merce", r)
	pageData.Data = map[string]interface{}{
		"DDTs":        ddts,
		"MeseFilter":  meseFilter,
		"AnnoFilter":  annoFilter,
	}

	renderTemplate(w, "amministrazione_ddt.html", pageData)
}

// DettaglioDDTAmministrazione mostra il dettaglio di un DDT
func DettaglioDDTAmministrazione(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/amministrazione/ddt", http.StatusSeeOther)
		return
	}
	ddtID, _ := strconv.ParseInt(pathParts[3], 10, 64)

	// Info DDT
	var ddt map[string]interface{}
	var id int
	var numero, tipoDdt, dataEmissione, nave, compagnia, porto, destinatario, indirizzo, vettore, note string
	err := database.DB.QueryRow(`
		SELECT d.id, d.numero, d.tipo_ddt, d.data_emissione,
		       COALESCE(n.nome, '') as nave, COALESCE(c.nome, '') as compagnia,
		       COALESCE(p.nome, '') as porto, COALESCE(d.destinatario, ''),
		       COALESCE(d.indirizzo, ''), COALESCE(d.vettore, ''), COALESCE(d.note, '')
		FROM ddt d
		LEFT JOIN navi n ON d.nave_id = n.id
		LEFT JOIN compagnie c ON d.compagnia_id = c.id
		LEFT JOIN porti p ON d.porto_id = p.id
		WHERE d.id = ?
	`, ddtID).Scan(&id, &numero, &tipoDdt, &dataEmissione, &nave, &compagnia, &porto, &destinatario, &indirizzo, &vettore, &note)
	if err != nil {
		http.Redirect(w, r, "/amministrazione/ddt", http.StatusSeeOther)
		return
	}

	ddt = map[string]interface{}{
		"ID":            id,
		"Numero":        numero,
		"TipoDDT":       tipoDdt,
		"DataEmissione": dataEmissione,
		"Nave":          nave,
		"Compagnia":     compagnia,
		"Porto":         porto,
		"Destinatario":  destinatario,
		"Indirizzo":     indirizzo,
		"Vettore":       vettore,
		"Note":          note,
	}

	// Righe DDT
	righeRows, _ := database.DB.Query(`
		SELECT r.quantita, COALESCE(r.descrizione, ''), p.codice, p.nome, p.unita_misura
		FROM righe_ddt r
		JOIN prodotti p ON r.prodotto_id = p.id
		WHERE r.ddt_id = ?
	`, ddtID)
	defer righeRows.Close()

	var righe []map[string]interface{}
	for righeRows.Next() {
		var quantita float64
		var descrizione, codice, nomeProdotto, unitaMisura string
		righeRows.Scan(&quantita, &descrizione, &codice, &nomeProdotto, &unitaMisura)
		righe = append(righe, map[string]interface{}{
			"Quantita":    quantita,
			"Descrizione": descrizione,
			"Codice":      codice,
			"Nome":        nomeProdotto,
			"UnitaMisura": unitaMisura,
		})
	}

	pageData := NewPageData("Dettaglio DDT", r)
	pageData.Data = map[string]interface{}{
		"DDT":   ddt,
		"Righe": righe,
	}

	renderTemplate(w, "amministrazione_ddt_dettaglio.html", pageData)
}

// ExportDDTCSV esporta i DDT in CSV
func ExportDDTCSV(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Error(w, "Non autorizzato", http.StatusUnauthorized)
		return
	}

	meseFilter := r.URL.Query().Get("mese")
	annoFilter := r.URL.Query().Get("anno")

	query := `
		SELECT d.numero, d.tipo_ddt, d.data_emissione,
		       COALESCE(n.nome, '') as nave, COALESCE(c.nome, '') as compagnia,
		       COALESCE(p.nome, '') as porto, COALESCE(d.destinatario, ''),
		       COALESCE(d.vettore, '')
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
	query += " ORDER BY d.data_emissione DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Errore esportazione", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	filename := fmt.Sprintf("ddt_%s.csv", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	writer.Comma = ';'
	writer.Write([]string{"Numero", "Tipo", "Data Emissione", "Nave", "Compagnia", "Porto", "Destinatario", "Vettore"})

	for rows.Next() {
		var numero, tipoDdt, dataEmissione, nave, compagnia, porto, destinatario, vettore string
		rows.Scan(&numero, &tipoDdt, &dataEmissione, &nave, &compagnia, &porto, &destinatario, &vettore)

		writer.Write([]string{
			numero, tipoDdt, dataEmissione, nave, compagnia, porto, destinatario, vettore,
		})
	}

	writer.Flush()
}
