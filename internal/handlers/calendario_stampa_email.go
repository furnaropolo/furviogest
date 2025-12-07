package handlers

import (
	"bytes"
	"fmt"
	"furviogest/internal/database"
	"furviogest/internal/middleware"
	"html/template"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"
	"log"
)

// SpesaDettaglio per la stampa
type SpesaDettaglio struct {
	Data            string
	DataFormattata  string
	TipoSpesa       string
	TipoSpesaLabel  string
	Importo         float64
	Note            string
	MetodoPagamento string
}

// StampaTrasferte genera il PDF/stampa del foglio trasferte
func StampaTrasferte(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	anno, _ := strconv.Atoi(r.URL.Query().Get("anno"))
	mese, _ := strconv.Atoi(r.URL.Query().Get("mese"))
	tecnicoID, _ := strconv.ParseInt(r.URL.Query().Get("tecnico"), 10, 64)

	if anno == 0 || mese == 0 {
		now := time.Now()
		anno = now.Year()
		mese = int(now.Month())
	}
	if tecnicoID == 0 {
		tecnicoID = session.UserID
	}
	if !session.IsTecnico() {
		tecnicoID = session.UserID
	}

	// Carica dati
	data := preparaDatiStampaTrasferte(tecnicoID, anno, mese)

	renderTemplate(w, "stampa_trasferte.html", PageData{
		Title:   "Stampa Trasferte",
		Session: session,
		Data:    data,
	})
}

// StampaNoteSpese genera il PDF/stampa della nota spese
func StampaNoteSpese(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	anno, _ := strconv.Atoi(r.URL.Query().Get("anno"))
	mese, _ := strconv.Atoi(r.URL.Query().Get("mese"))
	tecnicoID, _ := strconv.ParseInt(r.URL.Query().Get("tecnico"), 10, 64)

	if anno == 0 || mese == 0 {
		now := time.Now()
		anno = now.Year()
		mese = int(now.Month())
	}
	if tecnicoID == 0 {
		tecnicoID = session.UserID
	}
	if !session.IsTecnico() {
		tecnicoID = session.UserID
	}

	// Carica dati
	data := preparaDatiStampaNoteSpese(tecnicoID, anno, mese)

	renderTemplate(w, "stampa_note_spese.html", PageData{
		Title:   "Stampa Note Spese",
		Session: session,
		Data:    data,
	})
}

func preparaDatiStampaTrasferte(tecnicoID int64, anno, mese int) map[string]interface{} {
	// Nome tecnico
	var nomeTecnico string
	database.DB.QueryRow("SELECT cognome || ' ' || nome FROM utenti WHERE id = ?", tecnicoID).Scan(&nomeTecnico)

	// Dati azienda completi
	var ragioneSociale, piva, indirizzo, cap, citta, provincia, telefono, email, logo string
	database.DB.QueryRow(`
		SELECT COALESCE(ragione_sociale, ''), COALESCE(partita_iva, ''),
		       COALESCE(indirizzo, ''), COALESCE(cap, ''), COALESCE(citta, ''),
		       COALESCE(provincia, ''), COALESCE(telefono, ''), COALESCE(email, ''),
		       COALESCE(logo_path, '')
		FROM impostazioni_azienda WHERE id = 1
	`).Scan(&ragioneSociale, &piva, &indirizzo, &cap, &citta, &provincia, &telefono, &email, &logo)

	// Calcola festivi e giornate
	festivi := calcolaFestivi(anno, mese)
	giornateMap := caricaGiornateMese(tecnicoID, anno, mese)

	// Costruisci griglia
	primoGiorno := time.Date(anno, time.Month(mese), 1, 0, 0, 0, 0, time.Local)
	ultimoGiorno := primoGiorno.AddDate(0, 1, -1)

	primoDow := int(primoGiorno.Weekday())
	if primoDow == 0 {
		primoDow = 7
	}
	primoDow--

	var giorni []GiornoCalendario
	for i := 0; i < primoDow; i++ {
		giorni = append(giorni, GiornoCalendario{Giorno: 0})
	}

	for g := 1; g <= ultimoGiorno.Day(); g++ {
		dataStr := fmt.Sprintf("%04d-%02d-%02d", anno, mese, g)
		data := time.Date(anno, time.Month(mese), g, 0, 0, 0, 0, time.Local)
		isWeekend := data.Weekday() == time.Saturday || data.Weekday() == time.Sunday
		isFestivo := festivi[dataStr]

		gc := GiornoCalendario{
			Giorno:    g,
			Data:      dataStr,
			IsWeekend: isWeekend,
			IsFestivo: isFestivo,
		}

		if giornata, ok := giornateMap[dataStr]; ok {
			gc.TipoGiornata = giornata.TipoGiornata
			gc.Luogo = giornata.Luogo
			gc.NomeNave = giornata.NomeNave
		}

		giorni = append(giorni, gc)
	}

	// Riepilogo
	riepilogo := calcolaRiepilogoMese(tecnicoID, anno, mese)

	totaleTrasferte := int(riepilogo["giorni_trasferta_giornaliera"]) +
		int(riepilogo["giorni_trasferta_pernotto"]) +
		int(riepilogo["giorni_trasferta_festiva"])

	// Costruisci indirizzo completo
	indirizzoCompleto := indirizzo
	if cap != "" || citta != "" {
		indirizzoCompleto += " - " + cap + " " + citta
		if provincia != "" {
			indirizzoCompleto += " (" + provincia + ")"
		}
	}

	return map[string]interface{}{
		"Anno":            anno,
		"TecnicoID":       tecnicoID,
		"Mese":            mese,
		"NomeMese":        mesiItaliani[mese],
		"NomeTecnico":     nomeTecnico,
		"Azienda":         ragioneSociale,
		"AziendaIndirizzo": indirizzoCompleto,
		"AziendaPIVA":     piva,
		"AziendaTelefono": telefono,
		"AziendaEmail":    email,
		"AziendaLogo":     logo,
		"DataStampa":      time.Now().Format("02/01/2006"),
		"Giorni":          giorni,
		"RiepilogoGiorni": map[string]int{
			"ufficio":              int(riepilogo["giorni_ufficio"]),
			"trasferta_giornaliera": int(riepilogo["giorni_trasferta_giornaliera"]),
			"trasferta_pernotto":    int(riepilogo["giorni_trasferta_pernotto"]),
			"trasferta_festiva":     int(riepilogo["giorni_trasferta_festiva"]),
			"ferie":                 int(riepilogo["giorni_ferie"]),
		},
		"TotaleTrasferte": totaleTrasferte,
	}
}

func preparaDatiStampaNoteSpese(tecnicoID int64, anno, mese int) map[string]interface{} {
	// Nome tecnico
	var nomeTecnico string
	database.DB.QueryRow("SELECT cognome || ' ' || nome FROM utenti WHERE id = ?", tecnicoID).Scan(&nomeTecnico)

	// Dati azienda completi
	var ragioneSociale, piva, indirizzo, cap, citta, provincia, telefono, email, logo string
	database.DB.QueryRow(`
		SELECT COALESCE(ragione_sociale, ''), COALESCE(partita_iva, ''),
		       COALESCE(indirizzo, ''), COALESCE(cap, ''), COALESCE(citta, ''),
		       COALESCE(provincia, ''), COALESCE(telefono, ''), COALESCE(email, ''),
		       COALESCE(logo_path, '')
		FROM impostazioni_azienda WHERE id = 1
	`).Scan(&ragioneSociale, &piva, &indirizzo, &cap, &citta, &provincia, &telefono, &email, &logo)

	// Carica tutte le spese del mese
	query := `
		SELECT g.data, s.tipo_spesa, s.importo, COALESCE(s.note, ''), s.metodo_pagamento
		FROM spese_giornaliere s
		JOIN calendario_giornate g ON s.giornata_id = g.id
		WHERE g.tecnico_id = ? AND strftime('%Y', g.data) = ? AND strftime('%m', g.data) = ?
		ORDER BY g.data, s.id
	`
	rows, _ := database.DB.Query(query, tecnicoID, fmt.Sprintf("%04d", anno), fmt.Sprintf("%02d", mese))

	var spese []SpesaDettaglio
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var s SpesaDettaglio
			if err := rows.Scan(&s.Data, &s.TipoSpesa, &s.Importo, &s.Note, &s.MetodoPagamento); err != nil { log.Println("Errore scan spese:", err) }
			log.Printf("Spesa letta: Data=%s, Tipo=%s", s.Data, s.TipoSpesa)

			// Formatta data - prova diversi formati
			var t time.Time
			var err error
			if t, err = time.Parse("2006-01-02", s.Data); err != nil {
				t, err = time.Parse(time.RFC3339, s.Data)
			}
			if err == nil {
				s.DataFormattata = t.Format("02/01")
			}

			// Label tipo spesa
			labels := map[string]string{
				"carburante":   "Carburante",
				"cibo_hotel":   "Cibo/Hotel",
				"pedaggi_taxi": "Pedaggi/Taxi",
				"materiali":    "Materiali",
				"varie":        "Varie",
			}
			s.TipoSpesaLabel = labels[s.TipoSpesa]
			if s.TipoSpesaLabel == "" {
				s.TipoSpesaLabel = s.TipoSpesa
			}

			spese = append(spese, s)
		}
	}

	// Riepilogo
	riepilogo := calcolaRiepilogoMese(tecnicoID, anno, mese)

	totaleAziendale := riepilogo["totale_spese"] - riepilogo["totale_rimborso"]

	// Costruisci indirizzo completo
	indirizzoCompleto := indirizzo
	if cap != "" || citta != "" {
		indirizzoCompleto += " - " + cap + " " + citta
		if provincia != "" {
			indirizzoCompleto += " (" + provincia + ")"
		}
	}

	return map[string]interface{}{
		"TecnicoID":        tecnicoID,
		"Anno":             anno,
		"Mese":             mese,
		"NomeMese":         mesiItaliani[mese],
		"NomeTecnico":      nomeTecnico,
		"Azienda":          ragioneSociale,
		"AziendaIndirizzo": indirizzoCompleto,
		"AziendaPIVA":      piva,
		"AziendaTelefono":  telefono,
		"AziendaEmail":     email,
		"AziendaLogo":      logo,
		"DataStampa":       time.Now().Format("02/01/2006"),
		"Spese":            spese,
		"TotaleSpese":      riepilogo["totale_spese"],
		"TotaleRimborso":   riepilogo["totale_rimborso"],
		"TotaleAziendale":  totaleAziendale,
		"SpesePer": map[string]float64{
			"carburante":   riepilogo["carburante"],
			"cibo_hotel":   riepilogo["cibo_hotel"],
			"pedaggi_taxi": riepilogo["pedaggi_taxi"],
			"materiali":    riepilogo["materiali"],
			"varie":        riepilogo["varie"],
		},
	}
}

// InviaEmailTrasferte invia il foglio trasferte via email
func InviaEmailTrasferte(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Error(w, "Non autorizzato", http.StatusUnauthorized)
		return
	}

	anno, _ := strconv.Atoi(r.URL.Query().Get("anno"))
	mese, _ := strconv.Atoi(r.URL.Query().Get("mese"))
	tecnicoID, _ := strconv.ParseInt(r.URL.Query().Get("tecnico"), 10, 64)

	if anno == 0 || mese == 0 {
		now := time.Now()
		anno = now.Year()
		mese = int(now.Month())
	}
	if tecnicoID == 0 {
		tecnicoID = session.UserID
	}

	// Recupera destinatari
	var emailDest string
	database.DB.QueryRow("SELECT COALESCE(email_foglio_trasferte, '') FROM impostazioni_azienda WHERE id = 1").Scan(&emailDest)

	if emailDest == "" {
		http.Error(w, "Nessun destinatario configurato per il foglio trasferte", http.StatusBadRequest)
		return
	}

	// Prepara dati
	data := preparaDatiStampaTrasferte(tecnicoID, anno, mese)

	// Genera HTML
	htmlBody, err := generaHTMLEmail("stampa_trasferte.html", data)
	if err != nil {
		http.Error(w, "Errore generazione email: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Invia email
	subject := fmt.Sprintf("Foglio Trasferte - %s - %s %d", data["NomeTecnico"], data["NomeMese"], data["Anno"])
	err = inviaEmail(emailDest, subject, htmlBody)
	if err != nil {
		http.Error(w, "Errore invio email: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect con successo
	http.Redirect(w, r, fmt.Sprintf("/calendario-trasferte?anno=%d&mese=%d&tecnico=%d&email_sent=trasferte", anno, mese, tecnicoID), http.StatusSeeOther)
}

// InviaEmailNoteSpese invia la nota spese via email
func InviaEmailNoteSpese(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Error(w, "Non autorizzato", http.StatusUnauthorized)
		return
	}

	anno, _ := strconv.Atoi(r.URL.Query().Get("anno"))
	mese, _ := strconv.Atoi(r.URL.Query().Get("mese"))
	tecnicoID, _ := strconv.ParseInt(r.URL.Query().Get("tecnico"), 10, 64)

	if anno == 0 || mese == 0 {
		now := time.Now()
		anno = now.Year()
		mese = int(now.Month())
	}
	if tecnicoID == 0 {
		tecnicoID = session.UserID
	}

	// Recupera destinatari
	var emailDest string
	database.DB.QueryRow("SELECT COALESCE(email_nota_spese, '') FROM impostazioni_azienda WHERE id = 1").Scan(&emailDest)

	if emailDest == "" {
		http.Error(w, "Nessun destinatario configurato per la nota spese", http.StatusBadRequest)
		return
	}

	// Prepara dati
	data := preparaDatiStampaNoteSpese(tecnicoID, anno, mese)

	// Genera HTML
	htmlBody, err := generaHTMLEmail("stampa_note_spese.html", data)
	if err != nil {
		http.Error(w, "Errore generazione email: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Invia email
	subject := fmt.Sprintf("Nota Spese - %s - %s %d", data["NomeTecnico"], data["NomeMese"], data["Anno"])
	err = inviaEmail(emailDest, subject, htmlBody)
	if err != nil {
		http.Error(w, "Errore invio email: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect con successo
	http.Redirect(w, r, fmt.Sprintf("/calendario-trasferte?anno=%d&mese=%d&tecnico=%d&email_sent=spese", anno, mese, tecnicoID), http.StatusSeeOther)
}

func generaHTMLEmail(templateName string, data map[string]interface{}) (string, error) {
	// Carica template
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"mod": func(a, b int) int { return a % b },
	}

	tmpl, err := template.New(templateName).Funcs(funcMap).ParseFiles(
		"web/templates/" + templateName,
	)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	pageData := PageData{Data: data}
	err = tmpl.ExecuteTemplate(&buf, "content", pageData)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func inviaEmail(destinatari, subject, htmlBody string) error {
	// Recupera impostazioni SMTP
	var smtpServer, smtpUser, smtpPassword, smtpFromName string
	var smtpPort int
	database.DB.QueryRow(`
		SELECT COALESCE(smtp_server, ''), COALESCE(smtp_port, 587),
		       COALESCE(smtp_user, ''), COALESCE(smtp_password, ''),
		       COALESCE(smtp_from_name, ragione_sociale)
		FROM impostazioni_azienda WHERE id = 1
	`).Scan(&smtpServer, &smtpPort, &smtpUser, &smtpPassword, &smtpFromName)

	if smtpServer == "" || smtpUser == "" {
		return fmt.Errorf("SMTP non configurato")
	}

	// Prepara lista destinatari
	to := strings.Split(destinatari, ",")
	for i := range to {
		to[i] = strings.TrimSpace(to[i])
	}

	// Costruisci messaggio
	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("%s <%s>", smtpFromName, smtpUser)
	headers["To"] = strings.Join(to, ", ")
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	var msg bytes.Buffer
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)

	// Invia
	auth := smtp.PlainAuth("", smtpUser, smtpPassword, smtpServer)
	addr := fmt.Sprintf("%s:%d", smtpServer, smtpPort)

	return smtp.SendMail(addr, auth, smtpUser, to, msg.Bytes())
}
