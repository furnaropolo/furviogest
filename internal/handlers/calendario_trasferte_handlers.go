package handlers

import (
	"encoding/json"
	"fmt"
	"furviogest/internal/database"
	"furviogest/internal/middleware"
	"furviogest/internal/models"
	"net/http"
	"strconv"
	"time"
)

// GiornataCalendario rappresenta un giorno nel calendario
type GiornataCalendario struct {
	ID           int64
	TecnicoID    int64
	Data         string
	TipoGiornata string
	Luogo        string
	CompagniaID  *int64
	NaveID       *int64
	Note         string
	// Campi join
	NomeCompagnia string
	NomeNave      string
	// Spese del giorno
	Spese []SpesaGiornaliera
}

// SpesaGiornaliera rappresenta una spesa
type SpesaGiornaliera struct {
	ID               int64
	GiornataID       int64
	TipoSpesa        string
	Importo          float64
	Note             string
	MetodoPagamento  string
}

// CalendarioData contiene i dati per il template calendario
type CalendarioData struct {
	Anno           int
	Mese           int
	NomeMese       string
	Giorni         []GiornoCalendario
	Tecnici        []TecnicoInfo
	TecnicoID      int64
	NomeTecnico    string
	Compagnie      []models.Compagnia
	Navi           []models.Nave
	Festivi        map[string]bool
	TotaleSpese    float64
	TotaleRimborso float64
	SpesePer       map[string]float64
	RiepilogoGiorni map[string]int
	EmailSent       string
	IsAdmin         bool  // true se l'utente è admin e può selezionare altri tecnici
	Anni            []int // lista anni selezionabili (anno corrente ± 2)
	OrePermesso     int
	GiorniLavorativi int
}

// GiornoCalendario rappresenta un giorno nella griglia
type GiornoCalendario struct {
	Giorno        int
	Data          string
	TipoGiornata  string
	Colore        string
	IsFestivo     bool
	IsWeekend     bool
	GiornataID    int64
	Luogo         string
	NomeNave      string
	TotaleSpese   float64
}

// Nomi mesi italiani
var mesiItaliani = []string{
	"", "Gennaio", "Febbraio", "Marzo", "Aprile", "Maggio", "Giugno",
	"Luglio", "Agosto", "Settembre", "Ottobre", "Novembre", "Dicembre",
}

// CalendarioTrasferte mostra il calendario mensile
func CalendarioTrasferte(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Parametri filtro
	now := time.Now()
	anno, _ := strconv.Atoi(r.URL.Query().Get("anno"))
	mese, _ := strconv.Atoi(r.URL.Query().Get("mese"))
	tecnicoID, _ := strconv.ParseInt(r.URL.Query().Get("tecnico"), 10, 64)

	if anno == 0 {
		anno = now.Year()
	}
	if mese == 0 {
		mese = int(now.Month())
	}
	// Ogni utente vede di default il proprio calendario
	// Solo admin può selezionare altri tecnici
	isAdmin := session.Username == "admin"
	if tecnicoID == 0 || !isAdmin {
		tecnicoID = session.UserID
	}

	// Calcola festivi per anno/mese
	festivi := calcolaFestivi(anno, mese)

	// Carica giornate esistenti
	giornateMap := caricaGiornateMese(tecnicoID, anno, mese)
	fmt.Printf("DEBUG: tecnicoID=%d, anno=%d, mese=%d, giornate trovate=%d", tecnicoID, anno, mese, len(giornateMap))

	// Costruisci griglia calendario
	primoGiorno := time.Date(anno, time.Month(mese), 1, 0, 0, 0, 0, time.Local)
	ultimoGiorno := primoGiorno.AddDate(0, 1, -1)

	// Giorno settimana primo giorno (0=Dom, 1=Lun, ... adattiamo a Lun=0)
	primoDow := int(primoGiorno.Weekday())
	if primoDow == 0 {
		primoDow = 7
	}
	primoDow-- // Ora Lun=0, Mar=1, ..., Dom=6

	var giorni []GiornoCalendario

	// Celle vuote prima del primo giorno
	for i := 0; i < primoDow; i++ {
		giorni = append(giorni, GiornoCalendario{Giorno: 0})
	}

	// Giorni del mese
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

		// Cerca giornata esistente
		if giornata, ok := giornateMap[dataStr]; ok {
			gc.TipoGiornata = giornata.TipoGiornata
			gc.Colore = getColoreGiornata(giornata.TipoGiornata)
			fmt.Printf("DEBUG giorno %s: tipo=%s colore=%s\n", dataStr, giornata.TipoGiornata, gc.Colore)
			gc.GiornataID = giornata.ID
			gc.Luogo = giornata.Luogo
			gc.NomeNave = giornata.NomeNave
			gc.TotaleSpese = calcolaTotaleSpese(giornata.ID)
		} else {
			// Default: giorni lavorativi non impostati sono "ufficio"
			if !isWeekend && !isFestivo {
				gc.TipoGiornata = "ufficio"
				gc.Colore = "#ffffff"
			} else {
				gc.TipoGiornata = ""
				gc.Colore = ""
			}
		}

		giorni = append(giorni, gc)
	}

	// Carica lista tecnici (solo per admin)
	var tecnici []TecnicoInfo
	if isAdmin {
		tecnici, _ = getTecniciList()
	}

	// Nome tecnico selezionato
	var nomeTecnico string
	database.DB.QueryRow("SELECT cognome || ' ' || nome FROM utenti WHERE id = ?", tecnicoID).Scan(&nomeTecnico)

	// Carica compagnie e navi per modale
	compagnie := caricaCompagnieCalendario()
	navi, _ := caricaNavi()

	// Calcola riepilogo
	riepilogo := calcolaRiepilogoMese(tecnicoID, anno, mese)

	// Genera lista anni dinamica (anno corrente ± 2)
	annoCorrente := now.Year()
	anni := []int{annoCorrente - 2, annoCorrente - 1, annoCorrente, annoCorrente + 1, annoCorrente + 2}

	pageData := NewPageData("Calendario Trasferte", r)
	pageData.Data = CalendarioData{
		Anno:            anno,
		Mese:            mese,
		NomeMese:        mesiItaliani[mese],
		Giorni:          giorni,
		Tecnici:         tecnici,
		TecnicoID:       tecnicoID,
		NomeTecnico:     nomeTecnico,
		Compagnie:       compagnie,
		Navi:            navi,
		Festivi:         festivi,
		TotaleSpese:     riepilogo["totale_spese"],
		TotaleRimborso:  riepilogo["totale_rimborso"],
		SpesePer:        map[string]float64{
			"carburante":    riepilogo["carburante"],
			"cibo_hotel":    riepilogo["cibo_hotel"],
			"pedaggi_taxi":  riepilogo["pedaggi_taxi"],
			"materiali":     riepilogo["materiali"],
			"varie":         riepilogo["varie"],
		},
		RiepilogoGiorni: map[string]int{
			"ufficio":              int(riepilogo["giorni_ufficio"]),
			"trasferta_giornaliera": int(riepilogo["giorni_trasferta_giornaliera"]),
			"trasferta_pernotto":    int(riepilogo["notti_pernotto"]),
			"trasferta_festiva":     int(riepilogo["giorni_trasferta_festiva"]),
			"ferie":                 int(riepilogo["giorni_ferie"]),
		},
		EmailSent: r.URL.Query().Get("email_sent"),
		IsAdmin:   isAdmin,
		Anni:      anni,
		OrePermesso:     int(riepilogo["ore_permesso"]),
	}

	renderTemplate(w, "calendario_trasferte.html", pageData)
}

// caricaCompagnieCalendario carica tutte le compagnie
func caricaCompagnieCalendario() []models.Compagnia {
	var compagnie []models.Compagnia

	rows, err := database.DB.Query("SELECT id, nome FROM compagnie ORDER BY nome")
	if err != nil {
		return compagnie
	}
	defer rows.Close()

	for rows.Next() {
		var c models.Compagnia
		rows.Scan(&c.ID, &c.Nome)
		compagnie = append(compagnie, c)
	}

	return compagnie
}

// calcolaFestivi restituisce mappa dei giorni festivi per anno/mese
func calcolaFestivi(anno, mese int) map[string]bool {
	festivi := make(map[string]bool)

	// Festività fisse italiane
	festiviFissi := []struct{ m, g int }{
		{1, 1},   // Capodanno
		{1, 6},   // Epifania
		{4, 25},  // Liberazione
		{5, 1},   // Festa del Lavoro
		{6, 2},   // Festa della Repubblica
		{8, 15},  // Ferragosto
		{11, 1},  // Tutti i Santi
		{12, 8},  // Immacolata
		{12, 7},  // Sant'Ambrogio (Milano)
		{12, 25}, // Natale
		{12, 26}, // Santo Stefano
	}

	for _, f := range festiviFissi {
		if f.m == mese {
			dataStr := fmt.Sprintf("%04d-%02d-%02d", anno, mese, f.g)
			festivi[dataStr] = true
		}
	}

	// Pasqua e Pasquetta (calcolati)
	pasqua := calcolaPasqua(anno)
	pasquetta := pasqua.AddDate(0, 0, 1)

	if int(pasqua.Month()) == mese {
		festivi[pasqua.Format("2006-01-02")] = true
	}
	if int(pasquetta.Month()) == mese {
		festivi[pasquetta.Format("2006-01-02")] = true
	}

	// Domeniche
	primoGiorno := time.Date(anno, time.Month(mese), 1, 0, 0, 0, 0, time.Local)
	ultimoGiorno := primoGiorno.AddDate(0, 1, -1)
	for d := primoGiorno; !d.After(ultimoGiorno); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Sunday {
			festivi[d.Format("2006-01-02")] = true
		}
	}

	return festivi
}

// calcolaPasqua calcola la data di Pasqua con algoritmo di Gauss
func calcolaPasqua(anno int) time.Time {
	a := anno % 19
	b := anno / 100
	c := anno % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	meseP := (h + l - 7*m + 114) / 31
	giorno := ((h + l - 7*m + 114) % 31) + 1
	return time.Date(anno, time.Month(meseP), giorno, 0, 0, 0, 0, time.Local)
}

// getColoreGiornata restituisce il colore CSS per il tipo giornata
func getColoreGiornata(tipo string) string {
	switch tipo {
	case "ufficio":
		return "#ffffff"
	case "trasferta_giornaliera":
		return "#fff3cd" // giallo
	case "trasferta_pernotto":
		return "#d4edda" // verde
	case "trasferta_festiva":
		return "#f8d7da" // rosso
	case "ferie":
		return "#cce5ff" // blu
	case "permesso":
		return "#e2d5f7" // viola chiaro
	default:
		return ""
	}
}

// caricaGiornateMese carica le giornate esistenti per tecnico/mese
func caricaGiornateMese(tecnicoID int64, anno, mese int) map[string]GiornataCalendario {
	result := make(map[string]GiornataCalendario)

	query := `
		SELECT g.id, g.tecnico_id, g.data, g.tipo_giornata, COALESCE(g.luogo, ''),
		       g.compagnia_id, g.nave_id, COALESCE(g.note, ''),
		       COALESCE(c.nome, ''), COALESCE(n.nome, '')
		FROM calendario_giornate g
		LEFT JOIN compagnie c ON g.compagnia_id = c.id
		LEFT JOIN navi n ON g.nave_id = n.id
		WHERE g.tecnico_id = ? AND strftime('%Y', g.data) = ? AND strftime('%m', g.data) = ?
	`

	rows, err := database.DB.Query(query, tecnicoID, fmt.Sprintf("%04d", anno), fmt.Sprintf("%02d", mese))
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var g GiornataCalendario
		rows.Scan(&g.ID, &g.TecnicoID, &g.Data, &g.TipoGiornata, &g.Luogo,
			&g.CompagniaID, &g.NaveID, &g.Note, &g.NomeCompagnia, &g.NomeNave)
		// Estrai solo la parte data (YYYY-MM-DD) dal formato completo
		dataKey := g.Data
		if len(g.Data) > 10 {
			dataKey = g.Data[:10]
		}
		result[dataKey] = g
		fmt.Printf("DEBUG carica: data key=%s tipo=%s\n", g.Data, g.TipoGiornata)
	}

	return result
}

// calcolaTotaleSpese calcola totale spese per una giornata
func calcolaTotaleSpese(giornataID int64) float64 {
	var totale float64
	database.DB.QueryRow("SELECT COALESCE(SUM(importo), 0) FROM spese_giornaliere WHERE giornata_id = ?", giornataID).Scan(&totale)
	return totale
}

// calcolaRiepilogoMese calcola riepilogo mensile
func calcolaRiepilogoMese(tecnicoID int64, anno, mese int) map[string]float64 {
	riepilogo := make(map[string]float64)

	// Totale spese per categoria
	query := `
		SELECT COALESCE(s.tipo_spesa, ''), COALESCE(SUM(s.importo), 0)
		FROM spese_giornaliere s
		JOIN calendario_giornate g ON s.giornata_id = g.id
		WHERE g.tecnico_id = ? AND strftime('%Y', g.data) = ? AND strftime('%m', g.data) = ?
		GROUP BY s.tipo_spesa
	`
	rows, _ := database.DB.Query(query, tecnicoID, fmt.Sprintf("%04d", anno), fmt.Sprintf("%02d", mese))
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var tipo string
			var importo float64
			rows.Scan(&tipo, &importo)
			riepilogo[tipo] = importo
			riepilogo["totale_spese"] += importo
		}
	}

	// Totale da rimborsare
	query = `
		SELECT COALESCE(SUM(s.importo), 0)
		FROM spese_giornaliere s
		JOIN calendario_giornate g ON s.giornata_id = g.id
		WHERE g.tecnico_id = ? AND strftime('%Y', g.data) = ? AND strftime('%m', g.data) = ?
		AND s.metodo_pagamento = 'carta_personale'
	`
	var totaleRimborso float64
	database.DB.QueryRow(query, tecnicoID, fmt.Sprintf("%04d", anno), fmt.Sprintf("%02d", mese)).Scan(&totaleRimborso)
	riepilogo["totale_rimborso"] = totaleRimborso

	// Conteggio giorni per tipo
	query = `
		SELECT tipo_giornata, COUNT(*)
		FROM calendario_giornate
		WHERE tecnico_id = ? AND strftime('%Y', data) = ? AND strftime('%m', data) = ?
		GROUP BY tipo_giornata
	`
	rows2, _ := database.DB.Query(query, tecnicoID, fmt.Sprintf("%04d", anno), fmt.Sprintf("%02d", mese))
	if rows2 != nil {
		defer rows2.Close()
		for rows2.Next() {
			var tipo string
			var count int
			rows2.Scan(&tipo, &count)
			riepilogo["giorni_"+tipo] = float64(count)
		}
	}

	// Totale ore permesso
	var orePermesso int
	database.DB.QueryRow(`
		SELECT COALESCE(SUM(ore_permesso), 0)
		FROM calendario_giornate
		WHERE tecnico_id = ? AND strftime('%Y', data) = ? AND strftime('%m', data) = ?
	`, tecnicoID, fmt.Sprintf("%04d", anno), fmt.Sprintf("%02d", mese)).Scan(&orePermesso)
	riepilogo["ore_permesso"] = float64(orePermesso)

	// Calcola giorni lavorativi del mese (esclusi weekend e festivi)
	festivi := calcolaFestivi(anno, mese)
	primoGiorno := time.Date(anno, time.Month(mese), 1, 0, 0, 0, 0, time.Local)
	ultimoGiorno := primoGiorno.AddDate(0, 1, -1)
	giorniLavorativi := 0
	for g := 1; g <= ultimoGiorno.Day(); g++ {
		data := time.Date(anno, time.Month(mese), g, 0, 0, 0, 0, time.Local)
		dataStr := fmt.Sprintf("%04d-%02d-%02d", anno, mese, g)
		isWeekend := data.Weekday() == time.Saturday || data.Weekday() == time.Sunday
		isFestivo := festivi[dataStr]
		if !isWeekend && !isFestivo {
			giorniLavorativi++
		}
	}
	riepilogo["giorni_lavorativi"] = float64(giorniLavorativi)
	
	// Giorni ufficio = giorni lavorativi - giorni con altri tipi (escluso ufficio)
	altriGiorni := int(riepilogo["giorni_trasferta_giornaliera"] + riepilogo["giorni_trasferta_pernotto"] + riepilogo["giorni_trasferta_festiva"] + riepilogo["giorni_ferie"] + riepilogo["giorni_permesso"])
	riepilogo["giorni_ufficio"] = float64(giorniLavorativi - altriGiorni)

	// Calcola pernottamenti effettivi (notti, non giorni)
	riepilogo["notti_pernotto"] = float64(calcolaPernottamenti(tecnicoID, anno, mese))

	return riepilogo
}

// calcolaPernottamenti conta le notti effettive raggruppando i giorni consecutivi con pernotto
func calcolaPernottamenti(tecnicoID int64, anno, mese int) int {
	// Carica tutte le date con trasferta_pernotto ordinate
	query := `
		SELECT data FROM calendario_giornate
		WHERE tecnico_id = ? AND strftime('%Y', data) = ? AND strftime('%m', data) = ?
		AND tipo_giornata = 'trasferta_pernotto'
		ORDER BY data
	`
	rows, err := database.DB.Query(query, tecnicoID, fmt.Sprintf("%04d", anno), fmt.Sprintf("%02d", mese))
	if err != nil {
		return 0
	}
	defer rows.Close()

	var date []time.Time
	for rows.Next() {
		var dataStr string
		rows.Scan(&dataStr)
		// Parse data (può essere YYYY-MM-DD o con timestamp)
		if len(dataStr) > 10 {
			dataStr = dataStr[:10]
		}
		t, err := time.Parse("2006-01-02", dataStr)
		if err == nil {
			date = append(date, t)
		}
	}

	if len(date) == 0 {
		return 0
	}

	// Raggruppa giorni consecutivi e calcola notti
	totaleNotti := 0
	giorniNelBlocco := 1

	for i := 1; i < len(date); i++ {
		// Differenza in giorni tra data corrente e precedente
		diff := date[i].Sub(date[i-1]).Hours() / 24
		
		if diff == 1 {
			// Giorno consecutivo, stesso blocco
			giorniNelBlocco++
		} else {
			// Nuovo blocco - calcola notti del blocco precedente
			totaleNotti += giorniNelBlocco - 1
			giorniNelBlocco = 1
		}
	}
	
	// Aggiungi notti dell ultimo blocco
	totaleNotti += giorniNelBlocco - 1

	return totaleNotti
}

// SalvaGiornataReq struttura richiesta salvataggio giornata
// SalvaGiornataReq struttura richiesta salvataggio giornata
type SalvaGiornataReq struct {
	Data         string `json:"data"`
	TecnicoID    int64  `json:"tecnico_id"`
	TipoGiornata string `json:"tipo_giornata"`
	Luogo        string `json:"luogo"`
	CompagniaID  *int64 `json:"compagnia_id"`
	NaveID       *int64 `json:"nave_id"`
	Note         string `json:"note"`
	OrePermesso  int    `json:"ore_permesso"`
}

// API per salvare/aggiornare giornata
func APISalvaGiornata(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		w.Header().Set("Content-Type", "application/json"); w.WriteHeader(http.StatusUnauthorized); json.NewEncoder(w).Encode(map[string]string{"error": "Non autorizzato"})
		return
	}

	if r.Method != "POST" {
		w.Header().Set("Content-Type", "application/json"); w.WriteHeader(http.StatusMethodNotAllowed); json.NewEncoder(w).Encode(map[string]string{"error": "Metodo non permesso"})
		return
	}

	var req SalvaGiornataReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json"); w.WriteHeader(http.StatusBadRequest); json.NewEncoder(w).Encode(map[string]string{"error": "Errore parsing JSON"})
		return
	}

	// Se non tecnico, forza proprio ID
	if !session.IsTecnico() {
		req.TecnicoID = session.UserID
	}

	// Upsert giornata
	var giornataID int64
	err := database.DB.QueryRow("SELECT id FROM calendario_giornate WHERE tecnico_id = ? AND data = ?",
		req.TecnicoID, req.Data).Scan(&giornataID)

	if err != nil {
		// Insert
		result, err := database.DB.Exec(`
			INSERT INTO calendario_giornate (tecnico_id, data, tipo_giornata, luogo, compagnia_id, nave_id, note, ore_permesso)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, req.TecnicoID, req.Data, req.TipoGiornata, req.Luogo, req.CompagniaID, req.NaveID, req.Note, req.OrePermesso)
		if err != nil {
			w.Header().Set("Content-Type", "application/json"); w.WriteHeader(http.StatusInternalServerError); json.NewEncoder(w).Encode(map[string]string{"error": "Errore salvataggio: " + err.Error()})
			return
		}
		giornataID, _ = result.LastInsertId()
	} else {
		// Update
		_, err = database.DB.Exec(`
			UPDATE calendario_giornate
			SET tipo_giornata = ?, luogo = ?, compagnia_id = ?, nave_id = ?, note = ?, ore_permesso = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, req.TipoGiornata, req.Luogo, req.CompagniaID, req.NaveID, req.Note, req.OrePermesso, giornataID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json"); w.WriteHeader(http.StatusInternalServerError); json.NewEncoder(w).Encode(map[string]string{"error": "Errore aggiornamento: " + err.Error()})
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"giornata_id": giornataID,
		"colore":      getColoreGiornata(req.TipoGiornata),
	})
}

// API per caricare dettaglio giornata
func APIDettaglioGiornata(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		w.Header().Set("Content-Type", "application/json"); w.WriteHeader(http.StatusUnauthorized); json.NewEncoder(w).Encode(map[string]string{"error": "Non autorizzato"})
		return
	}

	tecnicoID, _ := strconv.ParseInt(r.URL.Query().Get("tecnico_id"), 10, 64)
	data := r.URL.Query().Get("data")

	if !session.IsTecnico() {
		tecnicoID = session.UserID
	}

	var g GiornataCalendario
	err := database.DB.QueryRow(`
		SELECT g.id, g.tecnico_id, g.data, g.tipo_giornata, COALESCE(g.luogo, ''),
		       g.compagnia_id, g.nave_id, COALESCE(g.note, '')
		FROM calendario_giornate g
		WHERE g.tecnico_id = ? AND g.data = ?
	`, tecnicoID, data).Scan(&g.ID, &g.TecnicoID, &g.Data, &g.TipoGiornata, &g.Luogo,
		&g.CompagniaID, &g.NaveID, &g.Note)

	if err != nil {
		// Giornata non esiste ancora
		g = GiornataCalendario{
			TecnicoID: tecnicoID,
			Data:      data,
		}
	} else {
		// Carica spese
		rows, _ := database.DB.Query(`
			SELECT id, giornata_id, tipo_spesa, importo, COALESCE(note, ''), metodo_pagamento
			FROM spese_giornaliere WHERE giornata_id = ?
		`, g.ID)
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var s SpesaGiornaliera
				rows.Scan(&s.ID, &s.GiornataID, &s.TipoSpesa, &s.Importo, &s.Note, &s.MetodoPagamento)
				g.Spese = append(g.Spese, s)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(g)
}

// SalvaSpesaReq struttura richiesta salvataggio spesa
type SalvaSpesaReq struct {
	GiornataID      int64   `json:"giornata_id"`
	TipoSpesa       string  `json:"tipo_spesa"`
	Importo         float64 `json:"importo"`
	Note            string  `json:"note"`
	OrePermesso  int    `json:"ore_permesso"`
	MetodoPagamento string  `json:"metodo_pagamento"`
}

// API per salvare spesa
func APISalvaSpesa(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		w.Header().Set("Content-Type", "application/json"); w.WriteHeader(http.StatusUnauthorized); json.NewEncoder(w).Encode(map[string]string{"error": "Non autorizzato"})
		return
	}

	if r.Method != "POST" {
		w.Header().Set("Content-Type", "application/json"); w.WriteHeader(http.StatusMethodNotAllowed); json.NewEncoder(w).Encode(map[string]string{"error": "Metodo non permesso"}); return
	}

	var req SalvaSpesaReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Errore parsing JSON"})
		return
	}

	result, err := database.DB.Exec(`
		INSERT INTO spese_giornaliere (giornata_id, tipo_spesa, importo, note, metodo_pagamento)
		VALUES (?, ?, ?, ?, ?)
	`, req.GiornataID, req.TipoSpesa, req.Importo, req.Note, req.MetodoPagamento)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Errore salvataggio spesa"})
		return
	}

	spesaID, _ := result.LastInsertId()
	totale := calcolaTotaleSpese(req.GiornataID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"spesa_id":      spesaID,
		"totale_giorno": totale,
	})
}

// EliminaSpesaReq struttura richiesta eliminazione spesa
type EliminaSpesaReq struct {
	SpesaID    int64 `json:"spesa_id"`
	GiornataID int64 `json:"giornata_id"`
}

// API per eliminare spesa
func APIEliminaSpesa(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		w.Header().Set("Content-Type", "application/json"); w.WriteHeader(http.StatusUnauthorized); json.NewEncoder(w).Encode(map[string]string{"error": "Non autorizzato"})
		return
	}

	if r.Method != "POST" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Metodo non permesso"})
		return
	}

	var req EliminaSpesaReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Errore parsing JSON"})
		return
	}

	database.DB.Exec("DELETE FROM spese_giornaliere WHERE id = ?", req.SpesaID)

	totale := calcolaTotaleSpese(req.GiornataID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"totale_giorno": totale,
	})
}

// API per navi di una compagnia
func APINaviCompagnia(w http.ResponseWriter, r *http.Request) {
	compagniaID, _ := strconv.ParseInt(r.URL.Query().Get("compagnia_id"), 10, 64)

	rows, err := database.DB.Query("SELECT id, nome FROM navi WHERE compagnia_id = ? ORDER BY nome", compagniaID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json"); w.WriteHeader(http.StatusInternalServerError); json.NewEncoder(w).Encode(map[string]string{"error": "Errore"}); return
	}
	defer rows.Close()

	var navi []map[string]interface{}
	for rows.Next() {
		var id int64
		var nome string
		rows.Scan(&id, &nome)
		navi = append(navi, map[string]interface{}{"id": id, "nome": nome})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(navi)
}

// EliminaGiornataReq struttura richiesta eliminazione giornata
type EliminaGiornataReq struct {
	GiornataID int64 `json:"giornata_id"`
}

// APIEliminaGiornata elimina una giornata e tutte le spese associate
func APIEliminaGiornata(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Non autorizzato"})
		return
	}

	if r.Method != "POST" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Metodo non permesso"})
		return
	}

	var req EliminaGiornataReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Errore parsing JSON"})
		return
	}

	// Prima elimina le spese associate
	database.DB.Exec("DELETE FROM spese_giornaliere WHERE giornata_id = ?", req.GiornataID)
	
	// Poi elimina la giornata
	database.DB.Exec("DELETE FROM calendario_giornate WHERE id = ?", req.GiornataID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}
