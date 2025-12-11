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
	"os"
	"os/exec"
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
			"trasferta_pernotto":    int(riepilogo["notti_pernotto"]),
			"trasferta_festiva":     int(riepilogo["giorni_trasferta_festiva"]),
			"ferie":                 int(riepilogo["giorni_ferie"]),
		},
		"TotaleTrasferte": totaleTrasferte,
		"OrePermesso":     int(riepilogo["ore_permesso"]),
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
		"euro": func(f float64) string { return strings.Replace(fmt.Sprintf("%.2f €", f), ".", ",", 1) },
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

// generaPDFConWkhtmltopdf genera un PDF da HTML usando wkhtmltopdf
func generaPDFConWkhtmltopdf(htmlContent string) ([]byte, error) {
	// Crea file HTML temporaneo
	tmpHTML, err := os.CreateTemp("", "pdf_*.html")
	if err != nil {
		return nil, fmt.Errorf("errore creazione file temp HTML: %v", err)
	}
	tmpHTMLName := tmpHTML.Name()
	defer os.Remove(tmpHTMLName)

	// Scrivi HTML
	_, err = tmpHTML.WriteString(htmlContent)
	tmpHTML.Close()
	if err != nil {
		return nil, fmt.Errorf("errore scrittura HTML: %v", err)
	}

	// Crea file PDF temporaneo
	tmpPDF, err := os.CreateTemp("", "pdf_*.pdf")
	if err != nil {
		return nil, fmt.Errorf("errore creazione file temp PDF: %v", err)
	}
	tmpPDFName := tmpPDF.Name()
	tmpPDF.Close()
	defer os.Remove(tmpPDFName)

	// Esegui wkhtmltopdf
	cmd := exec.Command("wkhtmltopdf",
		"--enable-local-file-access",
		"--page-size", "A4",
		"--margin-top", "10mm",
		"--margin-bottom", "10mm",
		"--margin-left", "15mm",
		"--margin-right", "15mm",
		"--encoding", "UTF-8",
		tmpHTMLName,
		tmpPDFName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("wkhtmltopdf error: %v, output: %s", err, string(output))
	}

	// Leggi PDF
	return os.ReadFile(tmpPDFName)
}

// generaPDFConFooter genera PDF con footer per numerazione pagine
func generaPDFConFooter(htmlContent string) ([]byte, error) {
	// Crea file HTML temporaneo
	tmpHTML, err := os.CreateTemp("", "pdf_*.html")
	if err != nil {
		return nil, fmt.Errorf("errore creazione file temp HTML: %v", err)
	}
	tmpHTMLName := tmpHTML.Name()
	defer os.Remove(tmpHTMLName)

	// Scrivi HTML
	_, err = tmpHTML.WriteString(htmlContent)
	tmpHTML.Close()
	if err != nil {
		return nil, fmt.Errorf("errore scrittura HTML: %v", err)
	}

	// Crea file PDF temporaneo
	tmpPDF, err := os.CreateTemp("", "pdf_*.pdf")
	if err != nil {
		return nil, fmt.Errorf("errore creazione file temp PDF: %v", err)
	}
	tmpPDFName := tmpPDF.Name()
	tmpPDF.Close()
	defer os.Remove(tmpPDFName)

	// Esegui wkhtmltopdf con footer per numerazione pagine
	cmd := exec.Command("wkhtmltopdf",
		"--enable-local-file-access",
		"--page-size", "A4",
		"--margin-top", "10mm",
		"--margin-bottom", "20mm",
		"--margin-left", "15mm",
		"--margin-right", "15mm",
		"--encoding", "UTF-8",
		"--footer-right", "Pag. [page]/[topage]",
		"--footer-font-size", "8",
		"--footer-spacing", "5",
		tmpHTMLName,
		tmpPDFName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("wkhtmltopdf error: %v, output: %s", err, string(output))
	}

	// Leggi PDF
	return os.ReadFile(tmpPDFName)
}

// DownloadPDFTrasferte genera e scarica il PDF delle trasferte usando wkhtmltopdf
func DownloadPDFTrasferte(w http.ResponseWriter, r *http.Request) {
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
	
	// Fix logo path per wkhtmltopdf - usa path assoluto
	if logoPath, ok := data["AziendaLogo"].(string); ok && logoPath != "" {
		if !strings.HasPrefix(logoPath, "/home/") {
			data["AziendaLogo"] = "/home/ies/furviogest/" + strings.TrimPrefix(logoPath, "/")
		}
	}

	// Genera HTML
	pageData := PageData{
		Title:   "Foglio Trasferte",
		Session: session,
		Data:    data,
	}

	// Render template
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"mod": func(a, b int) int { return a % b },
		"euro": func(f float64) string { return strings.Replace(fmt.Sprintf("%.2f €", f), ".", ",", 1) },
	}

	tmpl, err := template.New("trasferte_pdf.html").Funcs(funcMap).ParseFiles("web/templates/trasferte_pdf.html")
	if err != nil {
		log.Printf("Errore parse template trasferte PDF: %v", err)
		http.Error(w, "Errore template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "base", pageData)
	if err != nil {
		log.Printf("Errore execute template trasferte PDF: %v", err)
		http.Error(w, "Errore rendering: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Genera PDF con footer per numerazione pagine
	pdfData, err := generaPDFConFooter(buf.String())
	if err != nil {
		log.Printf("Errore generazione PDF trasferte: %v", err)
		http.Error(w, "Errore generazione PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Invia PDF
	nomeTecnico := data["NomeTecnico"].(string)
	nomeTecnico = strings.ReplaceAll(nomeTecnico, " ", "_")
	nomeMese := data["NomeMese"].(string)
	filename := fmt.Sprintf("Trasferte_%s_%s_%d.pdf", nomeTecnico, nomeMese, anno)

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfData)))
	w.Write(pdfData)
}

// DownloadPDFNoteSpese genera e scarica il PDF delle note spese usando wkhtmltopdf
func DownloadPDFNoteSpese(w http.ResponseWriter, r *http.Request) {
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

	// Fix logo path per wkhtmltopdf - usa path assoluto
	logoPath := ""
	if lp, ok := data["AziendaLogo"].(string); ok && lp != "" {
		if !strings.HasPrefix(lp, "/home/") {
			logoPath = "/home/ies/furviogest/" + strings.TrimPrefix(lp, "/")
		} else {
			logoPath = lp
		}
		data["AziendaLogo"] = logoPath
	}

	// Genera header HTML per ripetizione su ogni pagina
	headerHTML := generaHeaderNoteSpese(data, logoPath)

	// Genera HTML body
	pageData := PageData{
		Title:   "Nota Spese",
		Session: session,
		Data:    data,
	}

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"mod": func(a, b int) int { return a % b },
		"euro": func(f float64) string { return strings.Replace(fmt.Sprintf("%.2f €", f), ".", ",", 1) },
	}

	tmpl, err := template.New("notespese_pdf.html").Funcs(funcMap).ParseFiles("web/templates/notespese_pdf.html")
	if err != nil {
		log.Printf("Errore parse template note spese PDF: %v", err)
		http.Error(w, "Errore template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "base", pageData)
	if err != nil {
		log.Printf("Errore execute template note spese PDF: %v", err)
		http.Error(w, "Errore rendering: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Genera PDF con header ripetuto su ogni pagina
	pdfData, err := generaPDFConHeaderRipetuto(buf.String(), headerHTML)
	if err != nil {
		log.Printf("Errore generazione PDF note spese: %v", err)
		http.Error(w, "Errore generazione PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Invia PDF
	nomeTecnico := data["NomeTecnico"].(string)
	nomeTecnico = strings.ReplaceAll(nomeTecnico, " ", "_")
	nomeMese := data["NomeMese"].(string)
	filename := fmt.Sprintf("NoteSpese_%s_%s_%d.pdf", nomeTecnico, nomeMese, anno)

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfData)))
	w.Write(pdfData)
}

// generaHeaderNoteSpese genera l HTML dell header per le note spese
func generaHeaderNoteSpese(data map[string]interface{}, logoPath string) string {
	azienda := ""
	if a, ok := data["Azienda"].(string); ok {
		azienda = a
	}
	indirizzo := ""
	if i, ok := data["AziendaIndirizzo"].(string); ok {
		indirizzo = i
	}
	pivaTel := ""
	if p, ok := data["AziendaPIVA"].(string); ok && p != "" {
		pivaTel = "P.IVA: " + p
	}
	if t, ok := data["AziendaTelefono"].(string); ok && t != "" {
		if pivaTel != "" {
			pivaTel += " - "
		}
		pivaTel += "Tel: " + t
	}
	tecnico := ""
	if t, ok := data["NomeTecnico"].(string); ok {
		tecnico = t
	}
	periodo := ""
	if m, ok := data["NomeMese"].(string); ok {
		periodo = m
	}
	if a, ok := data["Anno"].(int); ok {
		periodo += fmt.Sprintf(" %d", a)
	}

	logoHTML := ""
	if logoPath != "" {
		logoHTML = fmt.Sprintf(`<img src="%s" alt="Logo">`, logoPath)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: Arial, sans-serif; font-size: 9pt; width: 100%%; }
        .header-container {
            display: table;
            width: 100%%;
            padding: 3mm 0;
            border-bottom: 2px solid #1565c0;
        }
        .header-left {
            display: table-cell;
            width: 15%%;
            vertical-align: middle;
        }
        .header-left img {
            max-width: 50px;
            max-height: 35px;
        }
        .header-center {
            display: table-cell;
            width: 50%%;
            vertical-align: middle;
        }
        .header-center .company-name {
            font-size: 11pt;
            font-weight: bold;
            color: #1565c0;
        }
        .header-center p {
            font-size: 7pt;
            color: #333;
            margin: 0;
        }
        .header-right {
            display: table-cell;
            width: 35%%;
            text-align: right;
            vertical-align: middle;
        }
        .header-right .title {
            font-size: 11pt;
            font-weight: bold;
            color: #1565c0;
        }
        .header-right p {
            font-size: 8pt;
            margin: 0;
        }
    </style>
</head>
<body>
    <div class="header-container">
        <div class="header-left">
            %s
        </div>
        <div class="header-center">
            <div class="company-name">%s</div>
            <p>%s</p>
            <p>%s</p>
        </div>
        <div class="header-right">
            <div class="title">NOTA SPESE - Pag. <span id="page"></span>/<span id="topage"></span></div>
            <p><strong>%s</strong></p>
            <p>%s</p>
        </div>
    </div>
    <script>
        var vars = {};
        var query_strings_from_url = document.location.search.substring(1).split("&");
        for (var query_string in query_strings_from_url) {
            if (query_strings_from_url.hasOwnProperty(query_string)) {
                var temp_var = query_strings_from_url[query_string].split("=");
                vars[temp_var[0]] = decodeURIComponent(temp_var[1]);
            }
        }
        document.getElementById("page").innerHTML = vars.page || "";
        document.getElementById("topage").innerHTML = vars.topage || "";
    </script>
</body>
</html>`, logoHTML, azienda, indirizzo, pivaTel, tecnico, periodo)
}

// generaPDFConHeader genera un PDF da HTML usando wkhtmltopdf con un header personalizzato
func generaPDFConHeader(htmlContent, headerHTML string) ([]byte, error) {
	// Crea file HTML temporaneo per il body
	tmpHTML, err := os.CreateTemp("", "pdf_body_*.html")
	if err != nil {
		return nil, fmt.Errorf("errore creazione file temp HTML: %v", err)
	}
	tmpHTMLName := tmpHTML.Name()
	defer os.Remove(tmpHTMLName)

	_, err = tmpHTML.WriteString(htmlContent)
	tmpHTML.Close()
	if err != nil {
		return nil, fmt.Errorf("errore scrittura HTML: %v", err)
	}

	// Crea file HTML temporaneo per header
	tmpHeader, err := os.CreateTemp("", "pdf_header_*.html")
	if err != nil {
		return nil, fmt.Errorf("errore creazione file temp header: %v", err)
	}
	tmpHeaderName := tmpHeader.Name()
	defer os.Remove(tmpHeaderName)

	_, err = tmpHeader.WriteString(headerHTML)
	tmpHeader.Close()
	if err != nil {
		return nil, fmt.Errorf("errore scrittura header: %v", err)
	}

	// Crea file PDF temporaneo
	tmpPDF, err := os.CreateTemp("", "pdf_*.pdf")
	if err != nil {
		return nil, fmt.Errorf("errore creazione file temp PDF: %v", err)
	}
	tmpPDFName := tmpPDF.Name()
	tmpPDF.Close()
	defer os.Remove(tmpPDFName)

	// Esegui wkhtmltopdf con header
	cmd := exec.Command("wkhtmltopdf",
		"--enable-local-file-access",
		"--page-size", "A4",
		"--margin-top", "25mm",
		"--margin-bottom", "20mm",
		"--margin-left", "15mm",
		"--margin-right", "15mm",
		"--encoding", "UTF-8",
		"--enable-javascript",
		"--javascript-delay", "100",
		"--header-html", tmpHeaderName,
		"--header-spacing", "5",
		tmpHTMLName,
		tmpPDFName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("wkhtmltopdf error: %v, output: %s", err, string(output))
	}

	// Leggi PDF
	return os.ReadFile(tmpPDFName)
}

// generaPDFConHeaderRipetuto genera PDF con header ripetuto su ogni pagina
func generaPDFConHeaderRipetuto(htmlContent string, headerHTML string) ([]byte, error) {
	// Crea file HTML temporaneo per il body
	tmpHTML, err := os.CreateTemp("", "pdf_body_*.html")
	if err != nil {
		return nil, fmt.Errorf("errore creazione file temp HTML: %v", err)
	}
	tmpHTMLName := tmpHTML.Name()
	defer os.Remove(tmpHTMLName)

	_, err = tmpHTML.WriteString(htmlContent)
	tmpHTML.Close()
	if err != nil {
		return nil, fmt.Errorf("errore scrittura HTML: %v", err)
	}

	// Crea file HTML temporaneo per header
	tmpHeader, err := os.CreateTemp("", "pdf_header_*.html")
	if err != nil {
		return nil, fmt.Errorf("errore creazione file temp header: %v", err)
	}
	tmpHeaderName := tmpHeader.Name()
	defer os.Remove(tmpHeaderName)

	_, err = tmpHeader.WriteString(headerHTML)
	tmpHeader.Close()
	if err != nil {
		return nil, fmt.Errorf("errore scrittura header: %v", err)
	}

	// Crea file PDF temporaneo
	tmpPDF, err := os.CreateTemp("", "pdf_*.pdf")
	if err != nil {
		return nil, fmt.Errorf("errore creazione file temp PDF: %v", err)
	}
	tmpPDFName := tmpPDF.Name()
	tmpPDF.Close()
	defer os.Remove(tmpPDFName)

	// Esegui wkhtmltopdf con header
	cmd := exec.Command("wkhtmltopdf",
		"--enable-local-file-access",
		"--enable-javascript",
		"--javascript-delay", "100",
		"--page-size", "A4",
		"--margin-top", "35mm",
		"--margin-bottom", "15mm",
		"--margin-left", "15mm",
		"--margin-right", "15mm",
		"--encoding", "UTF-8",
		"--header-html", tmpHeaderName,
		"--header-spacing", "5",
		tmpHTMLName,
		tmpPDFName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("wkhtmltopdf error: %v, output: %s", err, string(output))
	}

	// Leggi PDF
	return os.ReadFile(tmpPDFName)
}
