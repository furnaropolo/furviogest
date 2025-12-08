package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"furviogest/internal/database"
	"furviogest/internal/middleware"
)

// ============================================
// STRUTTURE DATI
// ============================================

type GuastoNave struct {
	ID                      int64
	NaveID                  int64
	NaveNome                string
	Tipo                    string // manuale o ap_fault
	APID                    *int64
	APNome                  string
	Gravita                 string // bassa, media, alta
	Descrizione             string
	Stato                   string // aperto, preso_in_carico, risolto
	TecnicoAperturaID       *int64
	TecnicoAperturaNome     string
	DataApertura            string
	TecnicoRisoluzioneID    *int64
	TecnicoRisoluzioneNome  string
	DataRisoluzione         string
	DescrizioneRisoluzione  string
}

type NaveGuastiCount struct {
	ID           int64
	Nome         string
	Compagnia    string
	GuastiAperti int
	GuastiAlti   int
}

type GuastiNavePageData struct {
	Nave    NaveInfo
	Guasti  []GuastoNave
	Tecnici []TecnicoSelect
}

type TecnicoSelect struct {
	ID   int64
	Nome string
}

type StoricoGuastiPageData struct {
	Guasti    []GuastoNave
	DataDa    string
	DataA     string
}

// ============================================
// LISTA NAVI CON CONTEGGIO GUASTI
// ============================================

func ListaNaviGuasti(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Segnalazione Guasti Nave - FurvioGest", r)

	rows, err := database.DB.Query(`
		SELECT n.id, n.nome, COALESCE(c.nome, '') as compagnia,
			(SELECT COUNT(*) FROM guasti_nave g WHERE g.nave_id = n.id AND g.stato != 'risolto') as guasti_aperti,
			(SELECT COUNT(*) FROM guasti_nave g WHERE g.nave_id = n.id AND g.stato != 'risolto' AND g.gravita = 'alta') as guasti_alti
		FROM navi n
		LEFT JOIN compagnie c ON n.compagnia_id = c.id
		ORDER BY guasti_alti DESC, guasti_aperti DESC, n.nome
	`)
	if err != nil {
		http.Error(w, "Errore caricamento navi", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var navi []NaveGuastiCount
	for rows.Next() {
		var n NaveGuastiCount
		rows.Scan(&n.ID, &n.Nome, &n.Compagnia, &n.GuastiAperti, &n.GuastiAlti)
		navi = append(navi, n)
	}

	data.Data = navi
	renderTemplate(w, "guasti_navi_lista.html", data)
}

// ============================================
// GUASTI SINGOLA NAVE
// ============================================

func GuastiNave(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/guasti-nave/")
	// Rimuovi eventuali sottopercorsi
	if idx := strings.Index(path, "/"); idx > 0 {
		path = path[:idx]
	}
	naveID, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/guasti-nave", http.StatusSeeOther)
		return
	}

	// Carica info nave
	var nave NaveInfo
	err = database.DB.QueryRow(`
		SELECT n.id, n.nome, COALESCE(c.nome, '') as compagnia, COALESCE(n.ferma_per_lavori, 0)
		FROM navi n
		LEFT JOIN compagnie c ON n.compagnia_id = c.id
		WHERE n.id = ?
	`, naveID).Scan(&nave.ID, &nave.Nome, &nave.NomeCompagnia, &nave.FermaPerLavori)
	if err != nil {
		http.Redirect(w, r, "/guasti-nave", http.StatusSeeOther)
		return
	}

	// Carica guasti non risolti
	guasti := getGuastiNave(naveID, false)

	// Carica tecnici per dropdown
	tecnici := getTecniciSelect()

	pageData := GuastiNavePageData{
		Nave:    nave,
		Guasti:  guasti,
		Tecnici: tecnici,
	}

	data := NewPageData("Guasti - "+nave.Nome, r)
	data.Data = pageData
	renderTemplate(w, "guasti_nave.html", data)
}

// ============================================
// NUOVO GUASTO
// ============================================

func NuovoGuasto(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metodo non consentito", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/guasti-nave/nuovo/")
	naveID, _ := strconv.ParseInt(path, 10, 64)

	r.ParseForm()
	gravita := r.FormValue("gravita")
	descrizione := strings.TrimSpace(r.FormValue("descrizione"))
	stato := r.FormValue("stato")
	if stato == "" {
		stato = "aperto"
	}

	// Ottieni ID tecnico dalla sessione
	session := middleware.GetSession(r)
	var tecnicoID interface{}
	if session != nil && session.UserID > 0 {
		tecnicoID = session.UserID
	}

	_, err := database.DB.Exec(`
		INSERT INTO guasti_nave (nave_id, tipo, gravita, descrizione, stato, tecnico_apertura_id)
		VALUES (?, 'manuale', ?, ?, ?, ?)
	`, naveID, gravita, descrizione, stato, tecnicoID)

	if err != nil {
		http.Error(w, "Errore salvataggio: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/guasti-nave/%d", naveID), http.StatusSeeOther)
}

// ============================================
// MODIFICA STATO GUASTO
// ============================================

func ModificaGuasto(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metodo non consentito", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/guasti-nave/modifica/")
	guastoID, _ := strconv.ParseInt(path, 10, 64)

	// Ottieni nave_id prima di modificare
	var naveID int64
	database.DB.QueryRow("SELECT nave_id FROM guasti_nave WHERE id = ?", guastoID).Scan(&naveID)

	r.ParseForm()
	stato := r.FormValue("stato")
	gravita := r.FormValue("gravita")
	descrizioneRisoluzione := strings.TrimSpace(r.FormValue("descrizione_risoluzione"))

	// Se risolto, imposta data e tecnico risoluzione
	if stato == "risolto" {
		session := middleware.GetSession(r)
		var tecnicoID interface{}
		if session != nil && session.UserID > 0 {
			tecnicoID = session.UserID
		}

		database.DB.Exec(`
			UPDATE guasti_nave 
			SET stato = ?, gravita = ?, descrizione_risoluzione = ?, 
			    tecnico_risoluzione_id = ?, data_risoluzione = CURRENT_TIMESTAMP,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, stato, gravita, descrizioneRisoluzione, tecnicoID, guastoID)
	} else {
		database.DB.Exec(`
			UPDATE guasti_nave 
			SET stato = ?, gravita = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, stato, gravita, guastoID)
	}

	http.Redirect(w, r, fmt.Sprintf("/guasti-nave/%d", naveID), http.StatusSeeOther)
}

// ============================================
// ELIMINA GUASTO
// ============================================

func EliminaGuasto(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/guasti-nave/elimina/")
	guastoID, _ := strconv.ParseInt(path, 10, 64)

	var naveID int64
	database.DB.QueryRow("SELECT nave_id FROM guasti_nave WHERE id = ?", guastoID).Scan(&naveID)

	database.DB.Exec("DELETE FROM guasti_nave WHERE id = ?", guastoID)

	http.Redirect(w, r, fmt.Sprintf("/guasti-nave/%d", naveID), http.StatusSeeOther)
}

// ============================================
// STORICO GUASTI
// ============================================

func StoricoGuasti(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Storico Guasti Nave - FurvioGest", r)

	// Parametri filtro date
	dataDa := r.URL.Query().Get("data_da")
	dataA := r.URL.Query().Get("data_a")

	// Default: ultimo mese
	if dataDa == "" {
		dataDa = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	}
	if dataA == "" {
		dataA = time.Now().Format("2006-01-02")
	}

	// Query guasti risolti nel periodo
	rows, err := database.DB.Query(`
		SELECT g.id, g.nave_id, n.nome as nave_nome, g.tipo, g.ap_id, 
		       COALESCE(ap.ap_name, '') as ap_nome, g.gravita, g.descrizione, g.stato,
		       g.tecnico_apertura_id, COALESCE(ua.nome || ' ' || ua.cognome, '') as tecnico_apertura,
		       g.data_apertura, g.tecnico_risoluzione_id, 
		       COALESCE(ur.nome || ' ' || ur.cognome, '') as tecnico_risoluzione,
		       COALESCE(g.data_risoluzione, '') as data_risoluzione,
		       COALESCE(g.descrizione_risoluzione, '') as descrizione_risoluzione
		FROM guasti_nave g
		JOIN navi n ON g.nave_id = n.id
		LEFT JOIN access_point ap ON g.ap_id = ap.id
		LEFT JOIN utenti ua ON g.tecnico_apertura_id = ua.id
		LEFT JOIN utenti ur ON g.tecnico_risoluzione_id = ur.id
		WHERE g.stato = 'risolto'
		AND DATE(g.data_risoluzione) >= DATE(?)
		AND DATE(g.data_risoluzione) <= DATE(?)
		ORDER BY g.data_risoluzione DESC
	`, dataDa, dataA)
	if err != nil {
		http.Error(w, "Errore caricamento storico", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var guasti []GuastoNave
	for rows.Next() {
		var g GuastoNave
		var apID sql.NullInt64
		var tecAperturaID, tecRisoluzioneID sql.NullInt64
		rows.Scan(&g.ID, &g.NaveID, &g.NaveNome, &g.Tipo, &apID, &g.APNome,
			&g.Gravita, &g.Descrizione, &g.Stato, &tecAperturaID, &g.TecnicoAperturaNome,
			&g.DataApertura, &tecRisoluzioneID, &g.TecnicoRisoluzioneNome,
			&g.DataRisoluzione, &g.DescrizioneRisoluzione)
		if apID.Valid {
			g.APID = &apID.Int64
		}
		guasti = append(guasti, g)
	}

	pageData := StoricoGuastiPageData{
		Guasti: guasti,
		DataDa: dataDa,
		DataA:  dataA,
	}

	data.Data = pageData
	renderTemplate(w, "guasti_storico.html", data)
}

// ============================================
// FUNZIONI HELPER
// ============================================

func getGuastiNave(naveID int64, includiRisolti bool) []GuastoNave {
	var guasti []GuastoNave

	query := `
		SELECT g.id, g.nave_id, n.nome as nave_nome, g.tipo, g.ap_id, 
		       COALESCE(ap.ap_name, '') as ap_nome, g.gravita, g.descrizione, g.stato,
		       g.tecnico_apertura_id, COALESCE(ua.nome || ' ' || ua.cognome, '') as tecnico_apertura,
		       g.data_apertura, g.tecnico_risoluzione_id, 
		       COALESCE(ur.nome || ' ' || ur.cognome, '') as tecnico_risoluzione,
		       COALESCE(g.data_risoluzione, '') as data_risoluzione,
		       COALESCE(g.descrizione_risoluzione, '') as descrizione_risoluzione
		FROM guasti_nave g
		JOIN navi n ON g.nave_id = n.id
		LEFT JOIN access_point ap ON g.ap_id = ap.id
		LEFT JOIN utenti ua ON g.tecnico_apertura_id = ua.id
		LEFT JOIN utenti ur ON g.tecnico_risoluzione_id = ur.id
		WHERE g.nave_id = ?
	`
	if !includiRisolti {
		query += " AND g.stato != 'risolto'"
	}
	query += " ORDER BY CASE g.gravita WHEN 'alta' THEN 1 WHEN 'media' THEN 2 ELSE 3 END, g.data_apertura DESC"

	rows, err := database.DB.Query(query, naveID)
	if err != nil {
		return guasti
	}
	defer rows.Close()

	for rows.Next() {
		var g GuastoNave
		var apID sql.NullInt64
		var tecAperturaID, tecRisoluzioneID sql.NullInt64
		rows.Scan(&g.ID, &g.NaveID, &g.NaveNome, &g.Tipo, &apID, &g.APNome,
			&g.Gravita, &g.Descrizione, &g.Stato, &tecAperturaID, &g.TecnicoAperturaNome,
			&g.DataApertura, &tecRisoluzioneID, &g.TecnicoRisoluzioneNome,
			&g.DataRisoluzione, &g.DescrizioneRisoluzione)
		if apID.Valid {
			g.APID = &apID.Int64
		}
		guasti = append(guasti, g)
	}
	return guasti
}

func getTecniciSelect() []TecnicoSelect {
	var tecnici []TecnicoSelect
	rows, err := database.DB.Query("SELECT id, nome || ' ' || cognome FROM utenti WHERE ruolo = 'tecnico' AND attivo = 1 ORDER BY cognome, nome")
	if err != nil {
		return tecnici
	}
	defer rows.Close()

	for rows.Next() {
		var t TecnicoSelect
		rows.Scan(&t.ID, &t.Nome)
		tecnici = append(tecnici, t)
	}
	return tecnici
}

// CreaGuastoAPFault crea automaticamente un guasto per un AP in fault
func CreaGuastoAPFault(naveID int64, apID int64, apNome string) {
	// Verifica se esiste già un guasto aperto per questo AP
	var existingID int64
	err := database.DB.QueryRow(`
		SELECT id FROM guasti_nave 
		WHERE nave_id = ? AND ap_id = ? AND tipo = 'ap_fault' AND stato != 'risolto'
	`, naveID, apID).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Non esiste, crea nuovo guasto
		database.DB.Exec(`
			INSERT INTO guasti_nave (nave_id, tipo, ap_id, gravita, descrizione, stato)
			VALUES (?, 'ap_fault', ?, 'alta', ?, 'aperto')
		`, naveID, apID, fmt.Sprintf("AP %s in stato FAULT - rilevato automaticamente", apNome))
	}
}

// ChiudiGuastoAPFault chiude automaticamente un guasto AP quando torna online
func ChiudiGuastoAPFault(naveID int64, apID int64) {
	database.DB.Exec(`
		UPDATE guasti_nave 
		SET stato = 'risolto', data_risoluzione = CURRENT_TIMESTAMP, 
		    descrizione_risoluzione = 'AP tornato online - chiuso automaticamente'
		WHERE nave_id = ? AND ap_id = ? AND tipo = 'ap_fault' AND stato != 'risolto'
	`, naveID, apID)
}
