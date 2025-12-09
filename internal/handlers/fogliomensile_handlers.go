package handlers

import (
	"net/http"
	"strconv"
	"time"

	"furviogest/internal/database"
	"furviogest/internal/middleware"
)

// RiepilogoMensileItem struttura per riepilogo mensile
type RiepilogoMensileItem struct {
	TecnicoID       int64
	NomeTecnico     string
	NumTrasferte    int
	TotaleKm        float64
	NumRapporti     int
	TotaleSpese     float64
	TotaleVitto     float64
	TotaleAlloggio  float64
	TotaleCarbur    float64
	TotalePedaggi   float64
	TotaleAltro     float64
}

// FoglioMensile mostra il foglio mensile per tecnici
func FoglioMensile(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Default al mese corrente
	now := time.Now()
	meseStr := r.URL.Query().Get("mese")
	annoStr := r.URL.Query().Get("anno")
	tecnicoFilter := r.URL.Query().Get("tecnico")

	if meseStr == "" {
		meseStr = now.Format("01")
	}
	if annoStr == "" {
		annoStr = now.Format("2006")
	}

	mese, _ := strconv.Atoi(meseStr)
	anno, _ := strconv.Atoi(annoStr)

	// Se non tecnico, vede solo i propri dati
	var tecnicoID int64
	if !session.IsTecnico() {
		tecnicoID = session.UserID
	} else if tecnicoFilter != "" {
		tecnicoID, _ = strconv.ParseInt(tecnicoFilter, 10, 64)
	}

	riepiloghi := getRiepilogoMensile(mese, anno, tecnicoID)
	dettaglioTrasferte := getDettaglioTrasferteMese(mese, anno, tecnicoID)
	dettaglioSpese := getDettaglioSpeseMese(mese, anno, tecnicoID)
	dettaglioRapporti := getDettaglioRapportiMese(mese, anno, tecnicoID)

	tecnici, _ := getTecniciList()

	pageData := NewPageData("Foglio Mensile", r)
	pageData.Data = map[string]interface{}{
		"Riepiloghi":          riepiloghi,
		"DettaglioTrasferte":  dettaglioTrasferte,
		"DettaglioSpese":      dettaglioSpese,
		"DettaglioRapporti":   dettaglioRapporti,
		"Tecnici":             tecnici,
		"MeseFilter":          meseStr,
		"AnnoFilter":          annoStr,
		"TecnicoFilter":       tecnicoFilter,
		"NomeMese":            getNomeMese(mese),
	}

	renderTemplate(w, "foglio_mensile.html", pageData)
}

// getRiepilogoMensile recupera riepilogo mensile per tecnici
func getRiepilogoMensile(mese, anno int, tecnicoID int64) []RiepilogoMensileItem {
	query := `
		SELECT u.id, u.cognome || ' ' || u.nome as nome,
		       (SELECT COUNT(*) FROM trasferte WHERE tecnico_id = u.id AND strftime('%m', data_partenza) = ? AND strftime('%Y', data_partenza) = ? AND deleted_at IS NULL) as num_trasferte,
		       (SELECT COALESCE(SUM(km_percorsi), 0) FROM trasferte WHERE tecnico_id = u.id AND strftime('%m', data_partenza) = ? AND strftime('%Y', data_partenza) = ? AND deleted_at IS NULL) as tot_km,
		       (SELECT COUNT(*) FROM tecnici_rapporto rt INNER JOIN rapporti_intervento r ON rt.rapporto_id = r.id WHERE rt.tecnico_id = u.id AND strftime('%m', r.data_intervento) = ? AND strftime('%Y', r.data_intervento) = ? AND r.deleted_at IS NULL) as num_rapporti,
		       (SELECT COALESCE(SUM(importo), 0) FROM note_spese WHERE tecnico_id = u.id AND strftime('%m', data) = ? AND strftime('%Y', data) = ? AND deleted_at IS NULL) as tot_spese,
		       (SELECT COALESCE(SUM(importo), 0) FROM note_spese WHERE tecnico_id = u.id AND tipo_spesa = 'vitto' AND strftime('%m', data) = ? AND strftime('%Y', data) = ? AND deleted_at IS NULL) as tot_vitto,
		       (SELECT COALESCE(SUM(importo), 0) FROM note_spese WHERE tecnico_id = u.id AND tipo_spesa = 'alloggio' AND strftime('%m', data) = ? AND strftime('%Y', data) = ? AND deleted_at IS NULL) as tot_alloggio,
		       (SELECT COALESCE(SUM(importo), 0) FROM note_spese WHERE tecnico_id = u.id AND tipo_spesa = 'carburante' AND strftime('%m', data) = ? AND strftime('%Y', data) = ? AND deleted_at IS NULL) as tot_carburante,
		       (SELECT COALESCE(SUM(importo), 0) FROM note_spese WHERE tecnico_id = u.id AND tipo_spesa = 'pedaggio' AND strftime('%m', data) = ? AND strftime('%Y', data) = ? AND deleted_at IS NULL) as tot_pedaggi,
		       (SELECT COALESCE(SUM(importo), 0) FROM note_spese WHERE tecnico_id = u.id AND tipo_spesa NOT IN ('vitto', 'alloggio', 'carburante', 'pedaggio') AND strftime('%m', data) = ? AND strftime('%Y', data) = ? AND deleted_at IS NULL) as tot_altro
		FROM utenti u
		WHERE u.ruolo = 'tecnico'
	`

	meseStr := strconv.Itoa(mese)
	if mese < 10 {
		meseStr = "0" + meseStr
	}
	annoStr := strconv.Itoa(anno)

	args := []interface{}{
		meseStr, annoStr, meseStr, annoStr, meseStr, annoStr, meseStr, annoStr,
		meseStr, annoStr, meseStr, annoStr, meseStr, annoStr, meseStr, annoStr, meseStr, annoStr,
	}

	if tecnicoID > 0 {
		query += " AND u.id = ?"
		args = append(args, tecnicoID)
	}

	query += " ORDER BY u.cognome, u.nome"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var riepiloghi []RiepilogoMensileItem
	for rows.Next() {
		var r RiepilogoMensileItem
		rows.Scan(&r.TecnicoID, &r.NomeTecnico, &r.NumTrasferte, &r.TotaleKm,
			&r.NumRapporti, &r.TotaleSpese, &r.TotaleVitto, &r.TotaleAlloggio,
			&r.TotaleCarbur, &r.TotalePedaggi, &r.TotaleAltro)
		riepiloghi = append(riepiloghi, r)
	}
	return riepiloghi
}

// getDettaglioTrasferteMese recupera dettaglio trasferte del mese
func getDettaglioTrasferteMese(mese, anno int, tecnicoID int64) []map[string]interface{} {
	meseStr := strconv.Itoa(mese)
	if mese < 10 {
		meseStr = "0" + meseStr
	}
	annoStr := strconv.Itoa(anno)

	query := `
		SELECT t.id, t.data_partenza, t.data_rientro, t.destinazione, t.km_percorsi,
		       COALESCE(u.cognome || ' ' || u.nome, '') as tecnico,
		       COALESCE(a.targa, '') as automezzo
		FROM trasferte t
		LEFT JOIN utenti u ON t.tecnico_id = u.id
		LEFT JOIN automezzi a ON t.automezzo_id = a.id
		WHERE strftime('%m', t.data_partenza) = ? AND strftime('%Y', t.data_partenza) = ?
		  AND t.deleted_at IS NULL
	`

	args := []interface{}{meseStr, annoStr}
	if tecnicoID > 0 {
		query += " AND t.tecnico_id = ?"
		args = append(args, tecnicoID)
	}
	query += " ORDER BY t.data_partenza DESC"

	rows, _ := database.DB.Query(query, args...)
	if rows == nil {
		return nil
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var id int64
		var dataP, dataR, dest, tecnico, auto string
		var km float64
		rows.Scan(&id, &dataP, &dataR, &dest, &km, &tecnico, &auto)
		result = append(result, map[string]interface{}{
			"ID": id, "DataPartenza": dataP, "DataRientro": dataR,
			"Destinazione": dest, "Km": km, "Tecnico": tecnico, "Automezzo": auto,
		})
	}
	return result
}

// getDettaglioSpeseMese recupera dettaglio spese del mese
func getDettaglioSpeseMese(mese, anno int, tecnicoID int64) []map[string]interface{} {
	meseStr := strconv.Itoa(mese)
	if mese < 10 {
		meseStr = "0" + meseStr
	}
	annoStr := strconv.Itoa(anno)

	query := `
		SELECT n.id, n.data, n.tipo_spesa, n.descrizione, n.importo, n.metodo_pagamento,
		       COALESCE(u.cognome || ' ' || u.nome, '') as tecnico
		FROM note_spese n
		LEFT JOIN utenti u ON n.tecnico_id = u.id
		WHERE strftime('%m', n.data) = ? AND strftime('%Y', n.data) = ?
		  AND n.deleted_at IS NULL
	`

	args := []interface{}{meseStr, annoStr}
	if tecnicoID > 0 {
		query += " AND n.tecnico_id = ?"
		args = append(args, tecnicoID)
	}
	query += " ORDER BY n.data DESC"

	rows, _ := database.DB.Query(query, args...)
	if rows == nil {
		return nil
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var id int64
		var data, tipo, desc, metodo, tecnico string
		var importo float64
		rows.Scan(&id, &data, &tipo, &desc, &importo, &metodo, &tecnico)
		result = append(result, map[string]interface{}{
			"ID": id, "Data": data, "Tipo": tipo, "Descrizione": desc,
			"Importo": importo, "MetodoPagamento": metodo, "Tecnico": tecnico,
		})
	}
	return result
}

// getDettaglioRapportiMese recupera rapporti del mese
func getDettaglioRapportiMese(mese, anno int, tecnicoID int64) []map[string]interface{} {
	meseStr := strconv.Itoa(mese)
	if mese < 10 {
		meseStr = "0" + meseStr
	}
	annoStr := strconv.Itoa(anno)

	query := `
		SELECT DISTINCT r.id, r.data_intervento, r.tipo_intervento, 
		       COALESCE(n.nome, '') as nave,
		       (SELECT GROUP_CONCAT(u.cognome || ' ' || u.nome, ', ')
		        FROM tecnici_rapporto rt
		        INNER JOIN utenti u ON rt.tecnico_id = u.id
		        WHERE rt.rapporto_id = r.id) as tecnici
		FROM rapporti_intervento r
		LEFT JOIN navi n ON r.nave_id = n.id
	`

	args := []interface{}{meseStr, annoStr}

	if tecnicoID > 0 {
		query += ` INNER JOIN tecnici_rapporto rt2 ON r.id = rt2.rapporto_id AND rt2.tecnico_id = ?`
		args = []interface{}{tecnicoID, meseStr, annoStr}
	}

	query += ` WHERE strftime('%m', r.data_intervento) = ? AND strftime('%Y', r.data_intervento) = ?
		   AND r.deleted_at IS NULL ORDER BY r.data_intervento DESC`

	rows, _ := database.DB.Query(query, args...)
	if rows == nil {
		return nil
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var id int64
		var data, tipo, nave, tecnici string
		rows.Scan(&id, &data, &tipo, &nave, &tecnici)
		result = append(result, map[string]interface{}{
			"ID": id, "Data": data, "Tipo": tipo, "Nave": nave, "Tecnici": tecnici,
		})
	}
	return result
}

// getNomeMese ritorna il nome del mese in italiano
func getNomeMese(mese int) string {
	nomi := []string{"", "Gennaio", "Febbraio", "Marzo", "Aprile", "Maggio", "Giugno",
		"Luglio", "Agosto", "Settembre", "Ottobre", "Novembre", "Dicembre"}
	if mese >= 1 && mese <= 12 {
		return nomi[mese]
	}
	return ""
}
