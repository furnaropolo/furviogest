package handlers

import (
	"fmt"
	"html/template"
	"furviogest/internal/email"
	"furviogest/internal/middleware"
	"database/sql"
	"furviogest/internal/database"
	"furviogest/internal/models"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// PermessoConDettagli contiene il permesso con i dati correlati
type PermessoConDettagli struct {
	models.RichiestaPermesso
	Tecnici    []models.Utente
	Automezzo  *models.Automezzo
	Porto      models.Porto
	Nave       models.Nave
	Compagnia  models.Compagnia
}

// ListaPermessi mostra l'elenco delle richieste permesso
func ListaPermessi(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Permessi Accesso Porto - FurvioGest", r)

	rows, err := database.DB.Query(`
		SELECT rp.id, rp.nave_id, rp.porto_id, rp.tecnico_creatore, rp.automezzo_id, 
			   rp.targa_esterna, rp.tipo_durata, rp.data_inizio, rp.data_fine,
			   rp.note, rp.email_inviata, rp.data_invio_email, rp.created_at,
			   n.nome as nome_nave, p.nome as nome_porto, 
			   u.nome || ' ' || u.cognome as nome_tecnico
		FROM richieste_permesso rp
		JOIN navi n ON rp.nave_id = n.id
		JOIN porti p ON rp.porto_id = p.id
		JOIN utenti u ON rp.tecnico_creatore = u.id
		ORDER BY rp.data_inizio DESC, rp.created_at DESC
	`)
	if err != nil {
		data.Error = "Errore nel recupero dei permessi: " + err.Error()
		renderTemplate(w, "permessi_lista.html", data)
		return
	}
	defer rows.Close()

	var permessi []models.RichiestaPermesso
	for rows.Next() {
		var p models.RichiestaPermesso
		var automezzoID sql.NullInt64
		var targaEsterna, note sql.NullString
		var dataFine, dataInvioEmail sql.NullTime

		err := rows.Scan(&p.ID, &p.NaveID, &p.PortoID, &p.TecnicoCreatore, &automezzoID,
			&targaEsterna, &p.TipoDurata, &p.DataInizio, &dataFine,
			&note, &p.EmailInviata, &dataInvioEmail, &p.CreatedAt,
			&p.NomeNave, &p.NomePorto, &p.NomeTecnico)
		if err != nil {
			continue
		}

		if automezzoID.Valid {
			id := automezzoID.Int64
			p.AutomezzoID = &id
		}
		if targaEsterna.Valid {
			p.TargaEsterna = targaEsterna.String
		}
		if note.Valid {
			p.Note = note.String
		}
		if dataFine.Valid {
			p.DataFine = &dataFine.Time
		}
		if dataInvioEmail.Valid {
			p.DataInvioEmail = &dataInvioEmail.Time
		}

		permessi = append(permessi, p)
	}

	data.Data = permessi
	renderTemplate(w, "permessi_lista.html", data)
}

// NuovoPermesso gestisce la creazione di una nuova richiesta permesso
func NuovoPermesso(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Nuova Richiesta Permesso - FurvioGest", r)

	// Carica dati per i dropdown
	formData := map[string]interface{}{}
	
	// Carica navi con compagnia
	navi, _ := caricaNavi()
	formData["Navi"] = navi

	// Carica porti
	porti, _ := caricaPorti()
	formData["Porti"] = porti

	// Carica tecnici attivi
	tecnici, _ := caricaTecniciAttivi()
	formData["Tecnici"] = tecnici

	// Carica automezzi
	automezzi, _ := caricaAutomezzi()
	formData["Automezzi"] = automezzi

	data.Data = formData

	if r.Method == http.MethodGet {
		renderTemplate(w, "permessi_form.html", data)
		return
	}

	// POST - salva il permesso
	r.ParseForm()

	naveID, err := strconv.ParseInt(r.FormValue("nave_id"), 10, 64)
	if err != nil {
		data.Error = "Seleziona una nave"
		renderTemplate(w, "permessi_form.html", data)
		return
	}

	portoID, err := strconv.ParseInt(r.FormValue("porto_id"), 10, 64)
	if err != nil {
		data.Error = "Seleziona un porto"
		renderTemplate(w, "permessi_form.html", data)
		return
	}

	tecnicoCreatore := data.Session.UserID

	var automezzoID *int64
	if aid := r.FormValue("automezzo_id"); aid != "" {
		id, err := strconv.ParseInt(aid, 10, 64)
		if err == nil {
			automezzoID = &id
		}
	}

	targaEsterna := strings.TrimSpace(r.FormValue("targa_esterna"))
	tipoDurata := models.TipoDurataPermesso(r.FormValue("tipo_durata"))
	
	dataInizio, err := time.Parse("2006-01-02", r.FormValue("data_inizio"))
	if err != nil {
		data.Error = "Data inizio non valida"
		renderTemplate(w, "permessi_form.html", data)
		return
	}

	var dataFine *time.Time
	if tipoDurata != models.DurataFineLavori {
		df, err := time.Parse("2006-01-02", r.FormValue("data_fine"))
		if err != nil {
			data.Error = "Data fine non valida"
			renderTemplate(w, "permessi_form.html", data)
			return
		}
		dataFine = &df
	}

	note := strings.TrimSpace(r.FormValue("note"))
	rientroInGiornata := r.FormValue("rientro_in_giornata") == "1"
	descrizioneIntervento := strings.TrimSpace(r.FormValue("descrizione_intervento"))
	tecniciSelezionati := r.Form["tecnici"]

	if len(tecniciSelezionati) == 0 {
		data.Error = "Seleziona almeno un tecnico"
		renderTemplate(w, "permessi_form.html", data)
		return
	}

	// Inserisci la richiesta permesso
	result, err := database.DB.Exec(`
		INSERT INTO richieste_permesso (nave_id, porto_id, tecnico_creatore, automezzo_id, 
			targa_esterna, tipo_durata, data_inizio, data_fine, note, descrizione_intervento, rientro_in_giornata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, naveID, portoID, tecnicoCreatore, automezzoID, targaEsterna, tipoDurata, dataInizio, dataFine, note, descrizioneIntervento, rientroInGiornata)

	if err != nil {
		data.Error = "Errore durante il salvataggio: " + err.Error()
		renderTemplate(w, "permessi_form.html", data)
		return
	}

	permessoID, _ := result.LastInsertId()

	// Inserisci i tecnici associati
	for _, tecnicoIDStr := range tecniciSelezionati {
		tecnicoID, err := strconv.ParseInt(tecnicoIDStr, 10, 64)
		if err == nil {
			database.DB.Exec(`
				INSERT INTO tecnici_permesso (richiesta_permesso_id, tecnico_id)
				VALUES (?, ?)
			`, permessoID, tecnicoID)
		}
	}


// 	// Se non e previsto il rientro in giornata, genera automaticamente le trasferte
// 	if !rientroInGiornata {
// 		// Calcola destinazione dal porto
// 		var destinazione string
// 		database.DB.QueryRow("SELECT nome || COALESCE(' - ' || citta, '') FROM porti WHERE id = ?", portoID).Scan(&destinazione)
// 		
// 		// Data rientro: se dataFine esiste usala, altrimenti usa dataInizio + 1
// 		dataRientro := dataInizio.AddDate(0, 0, 1)
// 		if dataFine != nil {
// 			dataRientro = *dataFine
// 		}
// 		
// // SCOLLEGATO: 		generaTrasfertePerPermesso(permessoID, tecniciSelezionati, destinazione, dataInizio, dataRientro, naveID, automezzoID)
// 	}

	http.Redirect(w, r, "/permessi", http.StatusSeeOther)
}

// ModificaPermesso gestisce la modifica di una richiesta permesso
func ModificaPermesso(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Modifica Richiesta Permesso - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/permessi", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/permessi", http.StatusSeeOther)
		return
	}

	// Carica dati per i dropdown
	formData := map[string]interface{}{}
	navi, _ := caricaNavi()
	formData["Navi"] = navi
	porti, _ := caricaPorti()
	formData["Porti"] = porti
	tecnici, _ := caricaTecniciAttivi()
	formData["Tecnici"] = tecnici
	automezzi, _ := caricaAutomezzi()
	formData["Automezzi"] = automezzi

	if r.Method == http.MethodGet {
		// Carica il permesso esistente
		var p models.RichiestaPermesso
		var automezzoID sql.NullInt64
		var targaEsterna, note sql.NullString
		var dataFine sql.NullTime

		err := database.DB.QueryRow(`
			SELECT id, nave_id, porto_id, automezzo_id, targa_esterna, 
				   tipo_durata, data_inizio, data_fine, note, rientro_in_giornata
			FROM richieste_permesso WHERE id = ?
		`, id).Scan(&p.ID, &p.NaveID, &p.PortoID, &automezzoID, &targaEsterna,
			&p.TipoDurata, &p.DataInizio, &dataFine, &note, &p.RientroInGiornata)

		if err != nil {
			http.Redirect(w, r, "/permessi", http.StatusSeeOther)
			return
		}

		if automezzoID.Valid {
			aid := automezzoID.Int64
			p.AutomezzoID = &aid
		}
		if targaEsterna.Valid {
			p.TargaEsterna = targaEsterna.String
		}
		if note.Valid {
			p.Note = note.String
		}
		if dataFine.Valid {
			p.DataFine = &dataFine.Time
		}

		// Carica tecnici selezionati
		var tecniciSelezionati []int64
		rows, _ := database.DB.Query(`
			SELECT tecnico_id FROM tecnici_permesso WHERE richiesta_permesso_id = ?
		`, id)
		defer rows.Close()
		for rows.Next() {
			var tid int64
			rows.Scan(&tid)
			tecniciSelezionati = append(tecniciSelezionati, tid)
		}

		formData["Permesso"] = p
		formData["TecniciSelezionati"] = tecniciSelezionati
		data.Data = formData
		renderTemplate(w, "permessi_form.html", data)
		return
	}

	// POST - aggiorna il permesso
	r.ParseForm()

	naveID, _ := strconv.ParseInt(r.FormValue("nave_id"), 10, 64)
	portoID, _ := strconv.ParseInt(r.FormValue("porto_id"), 10, 64)

	var automezzoID *int64
	if aid := r.FormValue("automezzo_id"); aid != "" {
		aidInt, err := strconv.ParseInt(aid, 10, 64)
		if err == nil {
			automezzoID = &aidInt
		}
	}

	targaEsterna := strings.TrimSpace(r.FormValue("targa_esterna"))
	tipoDurata := models.TipoDurataPermesso(r.FormValue("tipo_durata"))
	dataInizio, _ := time.Parse("2006-01-02", r.FormValue("data_inizio"))

	var dataFine *time.Time
	if tipoDurata != models.DurataFineLavori {
		df, err := time.Parse("2006-01-02", r.FormValue("data_fine"))
		if err == nil {
			dataFine = &df
		}
	}

	note := strings.TrimSpace(r.FormValue("note"))
	rientroInGiornata := r.FormValue("rientro_in_giornata") == "1"
	descrizioneIntervento := strings.TrimSpace(r.FormValue("descrizione_intervento"))
	tecniciSelezionati := r.Form["tecnici"]

	if len(tecniciSelezionati) == 0 {
		data.Error = "Seleziona almeno un tecnico"
		data.Data = formData
		renderTemplate(w, "permessi_form.html", data)
		return
	}

	// Aggiorna il permesso
	_, err = database.DB.Exec(`
		UPDATE richieste_permesso SET 
			nave_id = ?, porto_id = ?, automezzo_id = ?, targa_esterna = ?,
			tipo_durata = ?, data_inizio = ?, data_fine = ?, note = ?, descrizione_intervento = ?, rientro_in_giornata = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, naveID, portoID, automezzoID, targaEsterna, tipoDurata, dataInizio, dataFine, note, descrizioneIntervento, rientroInGiornata, id)

	if err != nil {
		data.Error = "Errore durante il salvataggio"
		data.Data = formData
		renderTemplate(w, "permessi_form.html", data)
		return
	}

	// Aggiorna tecnici associati
	database.DB.Exec("DELETE FROM tecnici_permesso WHERE richiesta_permesso_id = ?", id)
	for _, tecnicoIDStr := range tecniciSelezionati {
		tecnicoID, err := strconv.ParseInt(tecnicoIDStr, 10, 64)
		if err == nil {
			database.DB.Exec(`
				INSERT INTO tecnici_permesso (richiesta_permesso_id, tecnico_id)
				VALUES (?, ?)
			`, id, tecnicoID)
		}
	}

// 
// 	// Gestione trasferte in base a rientro_in_giornata
// 	if rientroInGiornata {
// 		// Se rientro in giornata, elimina eventuali trasferte collegate
// // SCOLLEGATO: 		eliminaTrasfertePermesso(id)
// 	} else {
// 		// Se non rientro in giornata, genera le trasferte
// 		var destinazione string
// 		database.DB.QueryRow("SELECT nome || COALESCE(' - ' || citta, '') FROM porti WHERE id = ?", portoID).Scan(&destinazione)
// 		
// 		dataRientro := dataInizio.AddDate(0, 0, 1)
// 		if dataFine != nil {
// 			dataRientro = *dataFine
// 		}
// 		
// 		// Prima elimina quelle vecchie poi rigenera
// // SCOLLEGATO: 		eliminaTrasfertePermesso(id)
// // SCOLLEGATO: 		generaTrasfertePerPermesso(id, tecniciSelezionati, destinazione, dataInizio, dataRientro, naveID, automezzoID)
// 	}
	http.Redirect(w, r, "/permessi", http.StatusSeeOther)
}

// EliminaPermesso elimina una richiesta permesso
func EliminaPermesso(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/permessi", http.StatusSeeOther)
		return
	}

	id, _ := strconv.ParseInt(pathParts[3], 10, 64)
	database.DB.Exec("DELETE FROM richieste_permesso WHERE id = ?", id)
	http.Redirect(w, r, "/permessi", http.StatusSeeOther)
}

// DettaglioPermesso mostra i dettagli di una richiesta permesso
func DettaglioPermesso(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Dettaglio Permesso - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/permessi", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/permessi", http.StatusSeeOther)
		return
	}

	// Carica permesso con tutti i dettagli
	var p models.RichiestaPermesso
	var automezzoID sql.NullInt64
	var targaEsterna, note sql.NullString
	var dataFine, dataInvioEmail sql.NullTime

	err = database.DB.QueryRow(`
		SELECT rp.id, rp.nave_id, rp.porto_id, rp.tecnico_creatore, rp.automezzo_id, 
			   rp.targa_esterna, rp.tipo_durata, rp.data_inizio, rp.data_fine,
			   rp.note, rp.email_inviata, rp.data_invio_email, rp.created_at,
			   n.nome, p.nome, u.nome || ' ' || u.cognome
		FROM richieste_permesso rp
		JOIN navi n ON rp.nave_id = n.id
		JOIN porti p ON rp.porto_id = p.id
		JOIN utenti u ON rp.tecnico_creatore = u.id
		WHERE rp.id = ?
	`, id).Scan(&p.ID, &p.NaveID, &p.PortoID, &p.TecnicoCreatore, &automezzoID,
		&targaEsterna, &p.TipoDurata, &p.DataInizio, &dataFine,
		&note, &p.EmailInviata, &dataInvioEmail, &p.CreatedAt,
		&p.NomeNave, &p.NomePorto, &p.NomeTecnico)

	if err != nil {
		http.Redirect(w, r, "/permessi", http.StatusSeeOther)
		return
	}

	if automezzoID.Valid {
		aid := automezzoID.Int64
		p.AutomezzoID = &aid
	}
	if targaEsterna.Valid {
		p.TargaEsterna = targaEsterna.String
	}
	if note.Valid {
		p.Note = note.String
	}
	if dataFine.Valid {
		p.DataFine = &dataFine.Time
	}
	if dataInvioEmail.Valid {
		p.DataInvioEmail = &dataInvioEmail.Time
	}

	dettagli := PermessoConDettagli{
		RichiestaPermesso: p,
	}

	// Carica tecnici associati
	rows, _ := database.DB.Query(`
		SELECT u.id, u.nome, u.cognome, u.email, u.telefono
		FROM utenti u
		JOIN tecnici_permesso tp ON u.id = tp.tecnico_id
		WHERE tp.richiesta_permesso_id = ?
	`, id)
	defer rows.Close()

	for rows.Next() {
		var t models.Utente
		var email, telefono sql.NullString
		rows.Scan(&t.ID, &t.Nome, &t.Cognome, &email, &telefono)
		if email.Valid {
			t.Email = email.String
		}
		if telefono.Valid {
			t.Telefono = telefono.String
		}
		dettagli.Tecnici = append(dettagli.Tecnici, t)
	}

	// Carica automezzo se presente
	if p.AutomezzoID != nil {
		var a models.Automezzo
		database.DB.QueryRow(`
			SELECT id, targa, marca, modello, COALESCE(libretto_path, '') FROM automezzi WHERE id = ?
		`, *p.AutomezzoID).Scan(&a.ID, &a.Targa, &a.Marca, &a.Modello, &a.LibrettoPath)
		dettagli.Automezzo = &a
	}

	// Carica porto con agenzia
	database.DB.QueryRow(`
		SELECT id, nome, citta, paese, nome_agenzia, email_agenzia, telefono_agenzia
		FROM porti WHERE id = ?
	`, p.PortoID).Scan(&dettagli.Porto.ID, &dettagli.Porto.Nome, &dettagli.Porto.Citta,
		&dettagli.Porto.Paese, &dettagli.Porto.NomeAgenzia, &dettagli.Porto.EmailAgenzia,
		&dettagli.Porto.TelefonoAgenzia)

	// Carica nave e compagnia
	database.DB.QueryRow(`
		SELECT n.id, n.nome, n.imo, c.id, c.nome
		FROM navi n
		JOIN compagnie c ON n.compagnia_id = c.id
		WHERE n.id = ?
	`, p.NaveID).Scan(&dettagli.Nave.ID, &dettagli.Nave.Nome, &dettagli.Nave.IMO,
		&dettagli.Compagnia.ID, &dettagli.Compagnia.Nome)

	data.Data = dettagli
	renderTemplate(w, "permessi_dettaglio.html", data)
}

// Funzioni helper per caricare dati dropdown
func caricaNavi() ([]models.Nave, error) {
	rows, err := database.DB.Query(`
		SELECT n.id, n.nome, n.imo, c.nome as nome_compagnia
		FROM navi n
		JOIN compagnie c ON n.compagnia_id = c.id
		ORDER BY c.nome, n.nome
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var navi []models.Nave
	for rows.Next() {
		var n models.Nave
		var imo sql.NullString
		rows.Scan(&n.ID, &n.Nome, &imo, &n.NomeCompagnia)
		if imo.Valid {
			n.IMO = imo.String
		}
		navi = append(navi, n)
	}
	return navi, nil
}

func caricaPorti() ([]models.Porto, error) {
	rows, err := database.DB.Query(`
		SELECT id, nome, citta, paese, nome_agenzia, email_agenzia
		FROM porti ORDER BY nome
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var porti []models.Porto
	for rows.Next() {
		var p models.Porto
		var citta, paese, nomeAgenzia, emailAgenzia sql.NullString
		rows.Scan(&p.ID, &p.Nome, &citta, &paese, &nomeAgenzia, &emailAgenzia)
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
		porti = append(porti, p)
	}
	return porti, nil
}

func caricaTecniciAttivi() ([]models.Utente, error) {
	rows, err := database.DB.Query(`
		SELECT id, nome, cognome, email, telefono
		FROM utenti WHERE attivo = 1 AND ruolo = 'tecnico'
		ORDER BY cognome, nome
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tecnici []models.Utente
	for rows.Next() {
		var t models.Utente
		var email, telefono sql.NullString
		rows.Scan(&t.ID, &t.Nome, &t.Cognome, &email, &telefono)
		if email.Valid {
			t.Email = email.String
		}
		if telefono.Valid {
			t.Telefono = telefono.String
		}
		tecnici = append(tecnici, t)
	}
	return tecnici, nil
}

func caricaAutomezzi() ([]models.Automezzo, error) {
	rows, err := database.DB.Query(`
		SELECT id, targa, marca, modello FROM automezzi ORDER BY targa
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var automezzi []models.Automezzo
	for rows.Next() {
		var a models.Automezzo
		var marca, modello sql.NullString
		rows.Scan(&a.ID, &a.Targa, &marca, &modello)
		if marca.Valid {
			a.Marca = marca.String
		}
		if modello.Valid {
			a.Modello = modello.String
		}
		automezzi = append(automezzi, a)
	}
	return automezzi, nil
}

// AnteprimaEmailPermesso mostra l'anteprima dell'email prima dell'invio
func AnteprimaEmailPermesso(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Anteprima Email Permesso - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/permessi", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/permessi", http.StatusSeeOther)
		return
	}

	permesso, err := caricaPermessoCompleto(id)
	if err != nil {
		data.Error = "Permesso non trovato"
		renderTemplate(w, "permessi_lista.html", data)
		return
	}

	impostazioni, _ := GetImpostazioniAziendaExport()

	session := middleware.GetSession(r)
	smtpConfigurato := false
	if session != nil {
		_, err := caricaSMTPTecnico(session.UserID)
		smtpConfigurato = (err == nil)
	}

	emailData := generaEmailDataPermesso(permesso, impostazioni)
	corpoHTML, err := email.GeneraCorpoEmailPermesso(emailData)
	if err != nil {
		data.Error = "Errore generazione email: " + err.Error()
		renderTemplate(w, "permessi_lista.html", data)
		return
	}

	oggetto := email.GeneraOggettoEmailPermesso(permesso.Nave.Nome, permesso.Porto.Nome, permesso.DataInizio)

	// Costruisci lista destinatari TO
	var destTO []string
	if permesso.Porto.EmailAgenzia != "" {
		destTO = append(destTO, permesso.Porto.EmailAgenzia)
	}
	// Solo per Grimaldi: aggiungi ispettore e master
	inviaATutti := permesso.Compagnia.EmailDestinatari == "tutti"
	if inviaATutti {
		if permesso.Nave.EmailIspettore != "" {
			destTO = append(destTO, permesso.Nave.EmailIspettore)
		}
		if permesso.Nave.EmailMaster != "" {
			destTO = append(destTO, permesso.Nave.EmailMaster)
		}
	}
	
	// Costruisci lista CC
	var destCC []string
	// Solo per Grimaldi: DDM in CC
	if inviaATutti && permesso.Nave.EmailDirettoreMacchina != "" {
		destCC = append(destCC, permesso.Nave.EmailDirettoreMacchina)
	}
	// Tecnici non mittenti sempre in CC
	for _, t := range permesso.Tecnici {
		if t.Email != "" && session != nil && t.ID != session.UserID {
			destCC = append(destCC, t.Email)
		}
	}

	previewData := map[string]interface{}{
		"PermessoID":      id,
		"DestinatariTO":   destTO,
		"DestinatariCC":   destCC,
		"NomeAgenzia":     permesso.Porto.NomeAgenzia,
		"Oggetto":         oggetto,
		"CorpoHTML":       template.HTML(corpoHTML),
		"SMTPConfigurato": smtpConfigurato,
		"Allegati":        []string{},
	}

	var allegati []string
	// Libretto automezzo
	if permesso.Automezzo != nil && permesso.Automezzo.LibrettoPath != "" {
		allegati = append(allegati, "Libretto " + permesso.Automezzo.Targa)
	}
	// Documenti tecnici
	for _, t := range permesso.Tecnici {
		if t.DocumentoPath != "" {
			allegati = append(allegati, "Documento "+t.Cognome+" "+t.Nome)
		}
	}
	previewData["Allegati"] = allegati

	data.Data = previewData
	renderTemplate(w, "permessi_anteprima_email.html", data)
}

// InviaEmailPermesso invia l'email per la richiesta permesso
func InviaEmailPermesso(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/permessi", http.StatusSeeOther)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/permessi", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/permessi", http.StatusSeeOther)
		return
	}

	permesso, err := caricaPermessoCompleto(id)
	if err != nil {
		http.Redirect(w, r, "/permessi?error=permesso_non_trovato", http.StatusSeeOther)
		return
	}

	if permesso.EmailInviata {
		http.Redirect(w, r, "/permessi/dettaglio/"+strconv.FormatInt(id, 10)+"?error=email_gia_inviata", http.StatusSeeOther)
		return
	}

	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	smtpConfig, err := caricaSMTPTecnico(session.UserID)
	if err != nil {
		http.Redirect(w, r, "/permessi/dettaglio/"+strconv.FormatInt(id, 10)+"?error=smtp_non_configurato", http.StatusSeeOther)
		return
	}

	impostazioni, _ := GetImpostazioniAziendaExport()

	emailDataPermesso := generaEmailDataPermesso(permesso, impostazioni)
	corpoHTML, err := email.GeneraCorpoEmailPermesso(emailDataPermesso)
	if err != nil {
		http.Redirect(w, r, "/permessi/dettaglio/"+strconv.FormatInt(id, 10)+"?error=errore_generazione", http.StatusSeeOther)
		return
	}

	oggetto := email.GeneraOggettoEmailPermesso(permesso.Nave.Nome, permesso.Porto.Nome, permesso.DataInizio)

	var allegati []email.Attachment
	for _, t := range permesso.Tecnici {
		if t.DocumentoPath != "" {
			att, err := email.CaricaAllegato(t.DocumentoPath)
			if err == nil {
				allegati = append(allegati, *att)
			}
		}
	}

	// Costruisci lista destinatari TO
	var destinatariTO []string
	if permesso.Porto.EmailAgenzia != "" {
		destinatariTO = append(destinatariTO, permesso.Porto.EmailAgenzia)
	}
	// Solo per Grimaldi: aggiungi ispettore e master
	inviaATutti := permesso.Compagnia.EmailDestinatari == "tutti"
	if inviaATutti {
		if permesso.Nave.EmailIspettore != "" {
			destinatariTO = append(destinatariTO, permesso.Nave.EmailIspettore)
		}
		if permesso.Nave.EmailMaster != "" {
			destinatariTO = append(destinatariTO, permesso.Nave.EmailMaster)
		}
	}

	// Costruisci lista CC
	var destinatariCC []string
	// Solo per Grimaldi: DDM in CC
	if inviaATutti && permesso.Nave.EmailDirettoreMacchina != "" {
		destinatariCC = append(destinatariCC, permesso.Nave.EmailDirettoreMacchina)
	}
	// Tecnici non mittenti sempre in CC
	for _, t := range permesso.Tecnici {
		if t.Email != "" && t.ID != session.UserID {
			destinatariCC = append(destinatariCC, t.Email)
		}
	}

	emailMsg := email.EmailData{
		To:          destinatariTO,
		Cc:          destinatariCC,
		Subject:     oggetto,
		HTMLBody:    corpoHTML,
		Attachments: allegati,
	}

	err = email.InviaEmail(*smtpConfig, emailMsg)
	if err != nil {
		http.Redirect(w, r, "/permessi/dettaglio/"+strconv.FormatInt(id, 10)+"?error=invio_fallito", http.StatusSeeOther)
		return
	}

	database.DB.Exec(`UPDATE richieste_permesso SET email_inviata = 1, data_invio_email = CURRENT_TIMESTAMP WHERE id = ?`, id)

	http.Redirect(w, r, "/permessi/dettaglio/"+strconv.FormatInt(id, 10)+"?success=email_inviata", http.StatusSeeOther)
}

// caricaPermessoCompleto carica tutti i dati di un permesso
func caricaPermessoCompleto(id int64) (*PermessoConDettagli, error) {
	var p models.RichiestaPermesso
	var automezzoID sql.NullInt64
	var targaEsterna, note, descrizioneIntervento sql.NullString
	var dataFine, dataInvioEmail sql.NullTime

	err := database.DB.QueryRow(`
		SELECT rp.id, rp.nave_id, rp.porto_id, rp.tecnico_creatore, rp.automezzo_id, 
			   rp.targa_esterna, rp.tipo_durata, rp.data_inizio, rp.data_fine,
			   rp.note, COALESCE(rp.descrizione_intervento, ''), rp.email_inviata, rp.data_invio_email, rp.created_at,
			   n.nome, p.nome, u.nome || ' ' || u.cognome
		FROM richieste_permesso rp
		JOIN navi n ON rp.nave_id = n.id
		JOIN porti p ON rp.porto_id = p.id
		JOIN utenti u ON rp.tecnico_creatore = u.id
		WHERE rp.id = ?
	`, id).Scan(&p.ID, &p.NaveID, &p.PortoID, &p.TecnicoCreatore, &automezzoID,
		&targaEsterna, &p.TipoDurata, &p.DataInizio, &dataFine,
		&note, &descrizioneIntervento, &p.EmailInviata, &dataInvioEmail, &p.CreatedAt,
		&p.NomeNave, &p.NomePorto, &p.NomeTecnico)

	if err != nil {
		return nil, err
	}

	if automezzoID.Valid {
		aid := automezzoID.Int64
		p.AutomezzoID = &aid
	}
	if targaEsterna.Valid {
		p.TargaEsterna = targaEsterna.String
	}
	if note.Valid {
		p.Note = note.String
	}
	if descrizioneIntervento.Valid {
		p.DescrizioneIntervento = descrizioneIntervento.String
	}
	if dataFine.Valid {
		p.DataFine = &dataFine.Time
	}
	if dataInvioEmail.Valid {
		p.DataInvioEmail = &dataInvioEmail.Time
	}

	dettagli := &PermessoConDettagli{RichiestaPermesso: p}

	// Carica tecnici
	rows, _ := database.DB.Query(`
		SELECT u.id, u.nome, u.cognome, u.email, u.telefono, u.documento_path
		FROM utenti u
		JOIN tecnici_permesso tp ON u.id = tp.tecnico_id
		WHERE tp.richiesta_permesso_id = ?
	`, id)
	defer rows.Close()
	for rows.Next() {
		var t models.Utente
		var emailU, telefono, docPath sql.NullString
		rows.Scan(&t.ID, &t.Nome, &t.Cognome, &emailU, &telefono, &docPath)
		if emailU.Valid {
			t.Email = emailU.String
		}
		if telefono.Valid {
			t.Telefono = telefono.String
		}
		if docPath.Valid {
			t.DocumentoPath = docPath.String
		}
		dettagli.Tecnici = append(dettagli.Tecnici, t)
	}

	// Carica automezzo
	if p.AutomezzoID != nil {
		var a models.Automezzo
		database.DB.QueryRow(`SELECT id, targa, marca, modello, COALESCE(libretto_path, '') FROM automezzi WHERE id = ?`, *p.AutomezzoID).Scan(&a.ID, &a.Targa, &a.Marca, &a.Modello, &a.LibrettoPath)
		dettagli.Automezzo = &a
	}

	// Carica porto
	var cittaPorto, paesePorto, nomeAgenzia, emailAgenzia, telefonoAgenzia sql.NullString
	database.DB.QueryRow(`SELECT id, nome, citta, paese, nome_agenzia, email_agenzia, telefono_agenzia FROM porti WHERE id = ?`, p.PortoID).Scan(
		&dettagli.Porto.ID, &dettagli.Porto.Nome, &cittaPorto, &paesePorto, &nomeAgenzia, &emailAgenzia, &telefonoAgenzia)
	if cittaPorto.Valid {
		dettagli.Porto.Citta = cittaPorto.String
	}
	if paesePorto.Valid {
		dettagli.Porto.Paese = paesePorto.String
	}
	if nomeAgenzia.Valid {
		dettagli.Porto.NomeAgenzia = nomeAgenzia.String
	}
	if emailAgenzia.Valid {
		dettagli.Porto.EmailAgenzia = emailAgenzia.String
	}
	if telefonoAgenzia.Valid {
		dettagli.Porto.TelefonoAgenzia = telefonoAgenzia.String
	}

	// Carica nave e compagnia con email
	var imoNave, emailMaster, emailDDM, emailIspettore sql.NullString
	database.DB.QueryRow(`
		SELECT n.id, n.nome, n.imo, n.email_master, n.email_direttore_macchina, n.email_ispettore, c.id, c.nome, COALESCE(c.email_destinatari, 'solo_agenzia')
		FROM navi n JOIN compagnie c ON n.compagnia_id = c.id WHERE n.id = ?
	`, p.NaveID).Scan(&dettagli.Nave.ID, &dettagli.Nave.Nome, &imoNave, &emailMaster, &emailDDM, &emailIspettore,
		&dettagli.Compagnia.ID, &dettagli.Compagnia.Nome)
	if imoNave.Valid {
		dettagli.Nave.IMO = imoNave.String
	}
	if emailMaster.Valid {
		dettagli.Nave.EmailMaster = emailMaster.String
	}
	if emailDDM.Valid {
		dettagli.Nave.EmailDirettoreMacchina = emailDDM.String
	}
	if emailIspettore.Valid {
		dettagli.Nave.EmailIspettore = emailIspettore.String
	}

	return dettagli, nil
}

// generaEmailDataPermesso genera i dati per il template email
func generaEmailDataPermesso(permesso *PermessoConDettagli, impostazioni *models.ImpostazioniAzienda) email.PermessoEmailData {
	tipoDurata := "Giornaliera"
	switch permesso.TipoDurata {
	case models.DurataMultigiorno:
		tipoDurata = "Multigiorno"
	case models.DurataFineLavori:
		tipoDurata = "Fino a Fine Lavori"
	}

	var tecnici []email.TecnicoEmail
	for _, t := range permesso.Tecnici {
		tecnici = append(tecnici, email.TecnicoEmail{
			NomeCognome: t.Cognome + " " + t.Nome,
			Email:       t.Email,
			Telefono:    t.Telefono,
		})
	}

	targa := ""
	if permesso.Automezzo != nil {
		targa = permesso.Automezzo.Targa
	} else if permesso.TargaEsterna != "" {
		targa = permesso.TargaEsterna
	}

	dataFine := ""
	if permesso.DataFine != nil {
		dataFine = permesso.DataFine.Format("02/01/2006")
	}

	return email.PermessoEmailData{
		RagioneSociale:        impostazioni.RagioneSociale,
		IndirizzoAz:           impostazioni.Indirizzo + ", " + impostazioni.CAP + " " + impostazioni.Citta,
		TelefonoAz:            impostazioni.Telefono,
		EmailAz:               impostazioni.Email,
		NomeNave:              permesso.Nave.Nome,
		IMO:                   permesso.Nave.IMO,
		NomeCompagnia:         permesso.Compagnia.Nome,
		NomePorto:             permesso.Porto.Nome,
		CittaPorto:            permesso.Porto.Citta,
		TipoDurata:            tipoDurata,
		DataInizio:            permesso.DataInizio.Format("02/01/2006"),
		DataFine:              dataFine,
		Tecnici:               tecnici,
		Targa:                 targa,
		DescrizioneIntervento: permesso.DescrizioneIntervento,
		Note:                  permesso.Note,
		FirmaTesto:            template.HTML(impostazioni.FirmaEmailTesto),
	}
}

// caricaSMTPTecnico carica le credenziali SMTP di un tecnico
func caricaSMTPTecnico(tecnicoID int64) (*email.SMTPConfig, error) {
	var smtpServer, smtpUser, smtpPassword, emailTecnico, nomeTecnico string
	var smtpPort int

	err := database.DB.QueryRow(`
		SELECT COALESCE(smtp_server, ''), COALESCE(smtp_port, 587), 
		       COALESCE(smtp_user, ''), COALESCE(smtp_password, ''),
		       COALESCE(email, ''), nome || ' ' || cognome
		FROM utenti WHERE id = ?
	`, tecnicoID).Scan(&smtpServer, &smtpPort, &smtpUser, &smtpPassword, &emailTecnico, &nomeTecnico)

	if err != nil {
		return nil, err
	}

	if smtpServer == "" {
		return nil, fmt.Errorf("SMTP non configurato")
	}

	if smtpPort == 0 {
		smtpPort = 587
	}

	fromAddr := smtpUser
	if fromAddr == "" {
		fromAddr = emailTecnico
	}

	return &email.SMTPConfig{
		Server:   smtpServer,
		Port:     smtpPort,
		User:     smtpUser,
		Password: smtpPassword,
		FromName: nomeTecnico,
		FromAddr: fromAddr,
	}, nil
}

// generaTrasfertePerPermesso crea automaticamente le trasferte per ogni tecnico
// quando il permesso non prevede il rientro in giornata
// quando il permesso non prevede il rientro in giornata
func generaTrasfertePerPermesso(permessoID int64, tecniciIDs []string, destinazione string, dataPartenza, dataRientro time.Time, naveID int64, automezzoID *int64) error {
	// Calcola numero notti
	giorni := int(dataRientro.Sub(dataPartenza).Hours()/24) + 1
	numeroNotti := giorni - 1
	if numeroNotti < 1 {
		numeroNotti = 1
	}

	for _, tecnicoIDStr := range tecniciIDs {
		tecnicoID, err := strconv.ParseInt(tecnicoIDStr, 10, 64)
		if err != nil {
			continue
		}

		// Verifica se esiste gia una trasferta per questo tecnico e permesso
		var count int
		database.DB.QueryRow("SELECT COUNT(*) FROM trasferte WHERE richiesta_permesso_id = ? AND tecnico_id = ? AND deleted_at IS NULL", permessoID, tecnicoID).Scan(&count)

		if count > 0 {
			continue // Trasferta gia esistente
		}

		// Crea la trasferta
		_, err = database.DB.Exec("INSERT INTO trasferte (tecnico_id, richiesta_permesso_id, destinazione, data_partenza, data_rientro, pernottamento, numero_notti, nave_id, automezzo_id, note) VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?, 'Generata automaticamente da richiesta permesso')", tecnicoID, permessoID, destinazione, dataPartenza, dataRientro, numeroNotti, naveID, automezzoID)

		if err != nil {
			return err
		}
	}
	return nil
}

// eliminaTrasfertePermesso elimina le trasferte collegate a un permesso
// quando il rientro in giornata viene cambiato a true
func eliminaTrasfertePermesso(permessoID int64) error {
	_, err := database.DB.Exec("UPDATE trasferte SET deleted_at = CURRENT_TIMESTAMP WHERE richiesta_permesso_id = ? AND deleted_at IS NULL", permessoID)
	return err
}
