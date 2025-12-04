package handlers

import (
	"furviogest/internal/auth"
	"net/http"
	"strconv"
	"strings"
	"time"

	"furviogest/internal/database"
	"furviogest/internal/middleware"
)

// Trasferta struttura trasferta
type Trasferta struct {
	ID            int64
	TecnicoID     int64
	RapportoID    *int64
	Destinazione  string
	DataPartenza  string
	DataRientro   string
	Pernottamento bool
	NumeroNotti   int
	KmPercorsi    int
	AutomezzoID   *int64
	Note          string
	// Campi join
	NomeTecnico   string
	NomeAutomezzo string
	TargaAuto     string
}

// ListaTrasferte mostra lista trasferte
func ListaTrasferte(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	tecnicoFilter := r.URL.Query().Get("tecnico")
	meseFilter := r.URL.Query().Get("mese")
	annoFilter := r.URL.Query().Get("anno")

	query := `
		SELECT t.id, t.tecnico_id, t.rapporto_id, t.destinazione, t.data_partenza, t.data_rientro,
		       t.pernottamento, t.numero_notti, COALESCE(t.km_percorsi, 0), t.automezzo_id,
		       COALESCE(t.note, ''),
		       COALESCE(u.cognome || ' ' || u.nome, '') as tecnico,
		       COALESCE(a.marca || ' ' || a.modello, '') as automezzo,
		       COALESCE(a.targa, '') as targa
		FROM trasferte t
		LEFT JOIN utenti u ON t.tecnico_id = u.id
		LEFT JOIN automezzi a ON t.automezzo_id = a.id
		WHERE t.deleted_at IS NULL
	`

	var args []interface{}

	// Se non tecnico, mostra solo le proprie
	if !session.IsTecnico() {
		query += " AND t.tecnico_id = ?"
		args = append(args, session.UserID)
	} else if tecnicoFilter != "" {
		query += " AND t.tecnico_id = ?"
		args = append(args, tecnicoFilter)
	}

	if meseFilter != "" && annoFilter != "" {
		query += " AND strftime('%m', t.data_partenza) = ? AND strftime('%Y', t.data_partenza) = ?"
		args = append(args, meseFilter, annoFilter)
	}

	query += " ORDER BY t.data_partenza DESC LIMIT 200"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Errore caricamento trasferte: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var trasferte []Trasferta
	for rows.Next() {
		var t Trasferta
		var pern int
		var rapID, autoID *int64
		err := rows.Scan(&t.ID, &t.TecnicoID, &rapID, &t.Destinazione, &t.DataPartenza, &t.DataRientro,
			&pern, &t.NumeroNotti, &t.KmPercorsi, &autoID, &t.Note,
			&t.NomeTecnico, &t.NomeAutomezzo, &t.TargaAuto)
		if err != nil {
			continue
		}
		t.Pernottamento = pern == 1
		t.RapportoID = rapID
		t.AutomezzoID = autoID
		trasferte = append(trasferte, t)
	}

	// Lista tecnici per filtro
	tecnici, _ := getTecniciList()

	pageData := NewPageData("Trasferte", r)
	pageData.Data = map[string]interface{}{
		"Trasferte":     trasferte,
		"Tecnici":       tecnici,
		"TecnicoFilter": tecnicoFilter,
		"MeseFilter":    meseFilter,
		"AnnoFilter":    annoFilter,
	}

	renderTemplate(w, "trasferte_lista.html", pageData)
}

// NuovaTrasferta gestisce creazione nuova trasferta
func NuovaTrasferta(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == "POST" {
		tecnicoID, _ := strconv.ParseInt(r.FormValue("tecnico_id"), 10, 64)
		rapportoIDStr := r.FormValue("rapporto_id")
		destinazione := r.FormValue("destinazione")
		dataPartenza := r.FormValue("data_partenza")
		dataRientro := r.FormValue("data_rientro")
		pernottamento := r.FormValue("pernottamento") == "1"
		numeroNotti, _ := strconv.Atoi(r.FormValue("numero_notti"))
		kmPercorsi, _ := strconv.Atoi(r.FormValue("km_percorsi"))
		automezzoIDStr := r.FormValue("automezzo_id")
		note := r.FormValue("note")

		// Se guest, forza tecnico_id al proprio
		if !session.IsTecnico() {
			tecnicoID = session.UserID
		}

		var rapportoID *int64
		if rapportoIDStr != "" {
			rid, _ := strconv.ParseInt(rapportoIDStr, 10, 64)
			rapportoID = &rid
		}

		var automezzoID *int64
		if automezzoIDStr != "" {
			aid, _ := strconv.ParseInt(automezzoIDStr, 10, 64)
			automezzoID = &aid
		}

		pernInt := 0
		if pernottamento {
			pernInt = 1
		}

		_, err := database.DB.Exec(`
			INSERT INTO trasferte (tecnico_id, rapporto_id, destinazione, data_partenza, data_rientro,
			                      pernottamento, numero_notti, km_percorsi, automezzo_id, note)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, tecnicoID, rapportoID, destinazione, dataPartenza, dataRientro,
			pernInt, numeroNotti, kmPercorsi, automezzoID, note)

		if err != nil {
			pageData := NewPageData("Nuova Trasferta", r)
			pageData.Error = "Errore creazione trasferta: " + err.Error()
			pageData.Data = getTrasfertaFormData(0, session)
			renderTemplate(w, "trasferta_form.html", pageData)
			return
		}

		http.Redirect(w, r, "/trasferte", http.StatusSeeOther)
		return
	}

	pageData := NewPageData("Nuova Trasferta", r)
	pageData.Data = getTrasfertaFormData(0, session)
	renderTemplate(w, "trasferta_form.html", pageData)
}

// ModificaTrasferta gestisce modifica trasferta
func ModificaTrasferta(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/trasferte/modifica/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/trasferte", http.StatusSeeOther)
		return
	}

	if r.Method == "POST" {
		tecnicoID, _ := strconv.ParseInt(r.FormValue("tecnico_id"), 10, 64)
		rapportoIDStr := r.FormValue("rapporto_id")
		destinazione := r.FormValue("destinazione")
		dataPartenza := r.FormValue("data_partenza")
		dataRientro := r.FormValue("data_rientro")
		pernottamento := r.FormValue("pernottamento") == "1"
		numeroNotti, _ := strconv.Atoi(r.FormValue("numero_notti"))
		kmPercorsi, _ := strconv.Atoi(r.FormValue("km_percorsi"))
		automezzoIDStr := r.FormValue("automezzo_id")
		note := r.FormValue("note")

		if !session.IsTecnico() {
			tecnicoID = session.UserID
		}

		var rapportoID *int64
		if rapportoIDStr != "" {
			rid, _ := strconv.ParseInt(rapportoIDStr, 10, 64)
			rapportoID = &rid
		}

		var automezzoID *int64
		if automezzoIDStr != "" {
			aid, _ := strconv.ParseInt(automezzoIDStr, 10, 64)
			automezzoID = &aid
		}

		pernInt := 0
		if pernottamento {
			pernInt = 1
		}

		_, err := database.DB.Exec(`
			UPDATE trasferte 
			SET tecnico_id = ?, rapporto_id = ?, destinazione = ?, data_partenza = ?, data_rientro = ?,
			    pernottamento = ?, numero_notti = ?, km_percorsi = ?, automezzo_id = ?, note = ?,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, tecnicoID, rapportoID, destinazione, dataPartenza, dataRientro,
			pernInt, numeroNotti, kmPercorsi, automezzoID, note, id)

		if err != nil {
			pageData := NewPageData("Modifica Trasferta", r)
			pageData.Error = "Errore modifica trasferta: " + err.Error()
			pageData.Data = getTrasfertaFormData(id, session)
			renderTemplate(w, "trasferta_form.html", pageData)
			return
		}

		http.Redirect(w, r, "/trasferte", http.StatusSeeOther)
		return
	}

	pageData := NewPageData("Modifica Trasferta", r)
	pageData.Data = getTrasfertaFormData(id, session)
	renderTemplate(w, "trasferta_form.html", pageData)
}

// EliminaTrasferta soft delete trasferta
func EliminaTrasferta(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/trasferte/elimina/")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	database.DB.Exec("UPDATE trasferte SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", id)

	http.Redirect(w, r, "/trasferte", http.StatusSeeOther)
}

func getTrasfertaFormData(trasfertaID int64, session *auth.Session) map[string]interface{} {
	data := make(map[string]interface{})

	// Lista tecnici (solo se tecnico)
	if session.IsTecnico(){
		tecnici, _ := getTecniciList()
		data["Tecnici"] = tecnici
	}

	// Lista automezzi
	automezzi, _ := getAutomezziList()
	data["Automezzi"] = automezzi

	// Lista rapporti recenti
	rapporti := getRapportiRecenti()
	data["Rapporti"] = rapporti

	// Trasferta esistente
	if trasfertaID > 0 {
		var t Trasferta
		var pern int
		var rapID, autoID *int64
		database.DB.QueryRow(`
			SELECT id, tecnico_id, rapporto_id, destinazione, data_partenza, data_rientro,
			       pernottamento, numero_notti, COALESCE(km_percorsi, 0), automezzo_id, COALESCE(note, '')
			FROM trasferte WHERE id = ?
		`, trasfertaID).Scan(&t.ID, &t.TecnicoID, &rapID, &t.Destinazione, &t.DataPartenza, &t.DataRientro,
			&pern, &t.NumeroNotti, &t.KmPercorsi, &autoID, &t.Note)
		t.Pernottamento = pern == 1
		t.RapportoID = rapID
		t.AutomezzoID = autoID
		data["Trasferta"] = t
	} else {
		data["Trasferta"] = Trasferta{
			TecnicoID:    session.UserID,
			DataPartenza: time.Now().Format("2006-01-02"),
			DataRientro:  time.Now().Format("2006-01-02"),
		}
	}

	return data
}

func getAutomezziList() ([]map[string]interface{}, error) {
	var automezzi []map[string]interface{}

	rows, err := database.DB.Query(`
		SELECT id, marca, modello, targa FROM automezzi 
		WHERE deleted_at IS NULL ORDER BY targa
	`)
	if err != nil {
		return automezzi, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var marca, modello, targa string
		rows.Scan(&id, &marca, &modello, &targa)
		automezzi = append(automezzi, map[string]interface{}{
			"ID":      id,
			"Nome":    marca + " " + modello,
			"Targa":   targa,
		})
	}

	return automezzi, nil
}

func getRapportiRecenti() []map[string]interface{} {
	var rapporti []map[string]interface{}

	rows, err := database.DB.Query(`
		SELECT r.id, r.data_intervento, COALESCE(n.nome, '')
		FROM rapporti_intervento r
		LEFT JOIN navi n ON r.nave_id = n.id
		WHERE r.deleted_at IS NULL
		ORDER BY r.data_intervento DESC
		LIMIT 50
	`)
	if err != nil {
		return rapporti
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var data, nave string
		rows.Scan(&id, &data, &nave)
		rapporti = append(rapporti, map[string]interface{}{
			"ID":   id,
			"Data": data,
			"Nave": nave,
		})
	}

	return rapporti
}
