package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"furviogest/internal/database"
	"furviogest/internal/middleware"
)

// RapportoIntervento struttura per rapporto
type RapportoIntervento struct {
	ID                  int64
	NaveID              int64
	PortoID             int64
	Tipo                string
	DataIntervento      string
	DataFine            string
	Descrizione         string
	Note                string
	ConsiderazioniFinali string
	PdfPath             string
	DDTGenerato         bool
	NumeroDDT           string
	CreatedAt           string
	// Campi join
	NomeNave            string
	NomeCompagnia       string
	NomePorto           string
}


// TecnicoRapporto per lista tecnici
type TecnicoRapporto struct {
	ID        int64
	Nome      string
	Cognome   string
	OreLavoro float64
	Selected  bool
}

// MaterialeRapporto per materiale usato (vecchia versione con magazzino)
type MaterialeRapporto struct {
	ID           int64
	ProdottoID   int64
	NomeProdotto string
	Codice       string
	Quantita     float64
	UnitaMisura  string
}

// MaterialeRapportoNew per materiale descrittivo (senza magazzino)
type MaterialeRapportoNew struct {
	ID                  int64
	Tipo                string // "utilizzato" o "recuperato"
	DescrizioneProdotto string
	Quantita            float64
	Unita               string
}

// FotoRapporto per foto allegate
type FotoRapporto struct {
	ID          int64
	FilePath    string
	Descrizione string
	CreatedAt   string
}

// ListaRapporti mostra la lista dei rapporti intervento
func ListaRapporti(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Filtri
	naveFilter := r.URL.Query().Get("nave")
	tipoFilter := r.URL.Query().Get("tipo")
	meseFilter := r.URL.Query().Get("mese")
	annoFilter := r.URL.Query().Get("anno")

	query := `
		SELECT r.id, r.nave_id, r.porto_id, r.tipo, r.data_intervento,
		       COALESCE(r.descrizione, ''), COALESCE(r.note, ''),
		       r.ddt_generato, COALESCE(r.numero_ddt, ''),
		       COALESCE(n.nome, '') as nave, COALESCE(c.nome, '') as compagnia,
		       COALESCE(p.nome, '') as porto
		FROM rapporti_intervento r
		LEFT JOIN navi n ON r.nave_id = n.id
		LEFT JOIN compagnie c ON n.compagnia_id = c.id
		LEFT JOIN porti p ON r.porto_id = p.id
		WHERE r.deleted_at IS NULL
	`

	var args []interface{}
	argIndex := 1

	if naveFilter != "" {
		query += fmt.Sprintf(" AND r.nave_id = ?%d", argIndex)
		args = append(args, naveFilter)
		argIndex++
	}
	if tipoFilter != "" {
		query += fmt.Sprintf(" AND r.tipo = ?%d", argIndex)
		args = append(args, tipoFilter)
		argIndex++
	}
	if meseFilter != "" && annoFilter != "" {
		query += " AND strftime('%m', r.data_intervento) = ? AND strftime('%Y', r.data_intervento) = ?"
		args = append(args, meseFilter, annoFilter)
	}

	query += " ORDER BY r.data_intervento DESC, r.id DESC LIMIT 200"

	// Fix query parameters
	query = strings.ReplaceAll(query, "?1", "?")
	query = strings.ReplaceAll(query, "?2", "?")
	query = strings.ReplaceAll(query, "?3", "?")
	query = strings.ReplaceAll(query, "?4", "?")

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Errore caricamento rapporti: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var rapporti []RapportoIntervento
	for rows.Next() {
		var r RapportoIntervento
		var ddtGen int
		err := rows.Scan(&r.ID, &r.NaveID, &r.PortoID, &r.Tipo, &r.DataIntervento,
			&r.Descrizione, &r.Note, &ddtGen, &r.NumeroDDT,
			&r.NomeNave, &r.NomeCompagnia, &r.NomePorto)
		if err != nil {
			continue
		}
		r.DDTGenerato = ddtGen == 1
		rapporti = append(rapporti, r)
	}

	// Lista navi per filtro
	navi, _ := getNaviList()

	pageData := NewPageData("Rapporti Intervento", r)
	pageData.Data = map[string]interface{}{
		"Rapporti":    rapporti,
		"Navi":        navi,
		"NaveFilter":  naveFilter,
		"TipoFilter":  tipoFilter,
		"MeseFilter":  meseFilter,
		"AnnoFilter":  annoFilter,
	}

	renderTemplate(w, "rapporti_lista.html", pageData)
}

// NuovoRapporto gestisce creazione nuovo rapporto
func NuovoRapporto(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == "POST" {
		naveID, _ := strconv.ParseInt(r.FormValue("nave_id"), 10, 64)
		portoID, _ := strconv.ParseInt(r.FormValue("porto_id"), 10, 64)
		tipo := r.FormValue("tipo")
		dataIntervento := r.FormValue("data_intervento")
		descrizione := r.FormValue("descrizione")
		note := r.FormValue("note")
		tecniciIDs := r.Form["tecnici"]

		// Inserisci rapporto
		result, err := database.DB.Exec(`
			INSERT INTO rapporti_intervento (nave_id, porto_id, tipo, data_intervento, descrizione, note)
			VALUES (?, ?, ?, ?, ?, ?)
		`, naveID, portoID, tipo, dataIntervento, descrizione, note)

		if err != nil {
			pageData := NewPageData("Nuovo Rapporto", r)
			pageData.Error = "Errore creazione rapporto: " + err.Error()
			pageData.Data = getRapportoFormData(0)
			renderTemplate(w, "rapporto_form.html", pageData)
			return
		}

		rapportoID, _ := result.LastInsertId()

		// Inserisci tecnici
		for _, tecID := range tecniciIDs {
			tid, _ := strconv.ParseInt(tecID, 10, 64)
			database.DB.Exec("INSERT INTO tecnici_rapporto (rapporto_id, tecnico_id) VALUES (?, ?)", rapportoID, tid)
		}

		http.Redirect(w, r, fmt.Sprintf("/rapporti/dettaglio/%d", rapportoID), http.StatusSeeOther)
		return
	}

	pageData := NewPageData("Nuovo Rapporto", r)
	pageData.Data = getRapportoFormData(0)
	renderTemplate(w, "rapporto_form.html", pageData)
}

// ModificaRapporto gestisce modifica rapporto
func ModificaRapporto(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/rapporti/modifica/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/rapporti", http.StatusSeeOther)
		return
	}

	if r.Method == "POST" {
		naveID, _ := strconv.ParseInt(r.FormValue("nave_id"), 10, 64)
		portoID, _ := strconv.ParseInt(r.FormValue("porto_id"), 10, 64)
		tipo := r.FormValue("tipo")
		dataIntervento := r.FormValue("data_intervento")
		descrizione := r.FormValue("descrizione")
		note := r.FormValue("note")
		tecniciIDs := r.Form["tecnici"]

		_, err := database.DB.Exec(`
			UPDATE rapporti_intervento 
			SET nave_id = ?, porto_id = ?, tipo = ?, data_intervento = ?, 
			    descrizione = ?, note = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, naveID, portoID, tipo, dataIntervento, descrizione, note, id)

		if err != nil {
			pageData := NewPageData("Modifica Rapporto", r)
			pageData.Error = "Errore modifica rapporto: " + err.Error()
			pageData.Data = getRapportoFormData(id)
			renderTemplate(w, "rapporto_form.html", pageData)
			return
		}

		// Aggiorna tecnici
		database.DB.Exec("DELETE FROM tecnici_rapporto WHERE rapporto_id = ?", id)
		for _, tecID := range tecniciIDs {
			tid, _ := strconv.ParseInt(tecID, 10, 64)
			database.DB.Exec("INSERT INTO tecnici_rapporto (rapporto_id, tecnico_id) VALUES (?, ?)", id, tid)
		}

		http.Redirect(w, r, fmt.Sprintf("/rapporti/dettaglio/%d", id), http.StatusSeeOther)
		return
	}

	pageData := NewPageData("Modifica Rapporto", r)
	pageData.Data = getRapportoFormData(id)
	renderTemplate(w, "rapporto_form.html", pageData)
}

// DettaglioRapporto mostra dettaglio rapporto
func DettaglioRapporto(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/rapporti/dettaglio/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/rapporti", http.StatusSeeOther)
		return
	}

	// Carica rapporto
	var rap RapportoIntervento
	var ddtGen int
	err = database.DB.QueryRow(`
		SELECT r.id, r.nave_id, r.porto_id, r.tipo, r.data_intervento,
		       COALESCE(r.descrizione, ''), COALESCE(r.note, ''),
		       r.ddt_generato, COALESCE(r.numero_ddt, ''),
		       COALESCE(n.nome, ''), COALESCE(c.nome, ''), COALESCE(p.nome, '')
		FROM rapporti_intervento r
		LEFT JOIN navi n ON r.nave_id = n.id
		LEFT JOIN compagnie c ON n.compagnia_id = c.id
		LEFT JOIN porti p ON r.porto_id = p.id
		WHERE r.id = ? AND r.deleted_at IS NULL
	`, id).Scan(&rap.ID, &rap.NaveID, &rap.PortoID, &rap.Tipo, &rap.DataIntervento,
		&rap.Descrizione, &rap.Note, &ddtGen, &rap.NumeroDDT,
		&rap.NomeNave, &rap.NomeCompagnia, &rap.NomePorto)

	if err != nil {
		http.Redirect(w, r, "/rapporti", http.StatusSeeOther)
		return
	}
	rap.DDTGenerato = ddtGen == 1

	// Carica tecnici
	tecnici := getTecniciRapporto(id)

	// Carica materiale
	materiali := getMaterialeRapporto(id)

	// Carica foto
	foto := getFotoRapporto(id)

	// Lista prodotti per aggiunta materiale
	prodotti, _ := getProdottiList()

	pageData := NewPageData("Dettaglio Rapporto #"+strconv.FormatInt(id, 10), r)
	pageData.Data = map[string]interface{}{
		"Rapporto":  rap,
		"Tecnici":   tecnici,
		"Materiali": materiali,
		"Foto":      foto,
		"Prodotti":  prodotti,
	}

	renderTemplate(w, "rapporto_dettaglio.html", pageData)
}

// EliminaRapporto soft delete rapporto
func EliminaRapporto(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/rapporti/elimina/")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	database.DB.Exec("UPDATE rapporti_intervento SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", id)

	http.Redirect(w, r, "/rapporti", http.StatusSeeOther)
}

// AggiungiMateriale aggiunge materiale al rapporto
func AggiungiMateriale(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method != "POST" {
		http.Redirect(w, r, "/rapporti", http.StatusSeeOther)
		return
	}

	rapportoID, _ := strconv.ParseInt(r.FormValue("rapporto_id"), 10, 64)
	prodottoID, _ := strconv.ParseInt(r.FormValue("prodotto_id"), 10, 64)
	quantita, _ := strconv.ParseFloat(r.FormValue("quantita"), 64)

	if quantita <= 0 {
		http.Redirect(w, r, fmt.Sprintf("/rapporti/dettaglio/%d?error=quantita", rapportoID), http.StatusSeeOther)
		return
	}

	// Verifica giacenza
	var giacenza float64
	database.DB.QueryRow("SELECT COALESCE(quantita, 0) FROM prodotti WHERE id = ?", prodottoID).Scan(&giacenza)
	if quantita > giacenza {
		http.Redirect(w, r, fmt.Sprintf("/rapporti/dettaglio/%d?error=giacenza", rapportoID), http.StatusSeeOther)
		return
	}

	// Inserisci materiale
	database.DB.Exec(`INSERT INTO materiale_rapporto (rapporto_id, prodotto_id, quantita) VALUES (?, ?, ?)`,
		rapportoID, prodottoID, quantita)

	// Scarica da magazzino
	database.DB.Exec("UPDATE prodotti SET quantita = quantita - ? WHERE id = ?", quantita, prodottoID)

	// Registra movimento magazzino
	database.DB.Exec(`
		INSERT INTO movimenti_magazzino (prodotto_id, tipo, quantita, motivo, utente_id)
		VALUES (?, 'scarico', ?, ?, ?)
	`, prodottoID, quantita, fmt.Sprintf("Rapporto intervento #%d", rapportoID), session.UserID)

	http.Redirect(w, r, fmt.Sprintf("/rapporti/dettaglio/%d", rapportoID), http.StatusSeeOther)
}

// RimuoviMateriale rimuove materiale dal rapporto
func RimuoviMateriale(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/rapporti/materiale/rimuovi/")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	// Recupera dati materiale prima di eliminare
	var rapportoID, prodottoID int64
	var quantita float64
	database.DB.QueryRow("SELECT rapporto_id, prodotto_id, quantita FROM materiale_rapporto WHERE id = ?", id).
		Scan(&rapportoID, &prodottoID, &quantita)

	// Elimina materiale
	database.DB.Exec("DELETE FROM materiale_rapporto WHERE id = ?", id)

	// Ricarica magazzino
	database.DB.Exec("UPDATE prodotti SET quantita = quantita + ? WHERE id = ?", quantita, prodottoID)

	// Registra movimento
	database.DB.Exec(`
		INSERT INTO movimenti_magazzino (prodotto_id, tipo, quantita, motivo, utente_id)
		VALUES (?, 'carico', ?, ?, ?)
	`, prodottoID, quantita, fmt.Sprintf("Reso da rapporto #%d", rapportoID), session.UserID)

	http.Redirect(w, r, fmt.Sprintf("/rapporti/dettaglio/%d", rapportoID), http.StatusSeeOther)
}

// UploadFotoRapporto carica foto per rapporto
func UploadFotoRapporto(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method != "POST" {
		http.Redirect(w, r, "/rapporti", http.StatusSeeOther)
		return
	}

	rapportoID, _ := strconv.ParseInt(r.FormValue("rapporto_id"), 10, 64)
	descrizione := r.FormValue("descrizione")

	// Parse multipart form
	err := r.ParseMultipartForm(32 << 20) // 32MB max
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/rapporti/dettaglio/%d?error=upload", rapportoID), http.StatusSeeOther)
		return
	}

	file, header, err := r.FormFile("foto")
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/rapporti/dettaglio/%d?error=nofile", rapportoID), http.StatusSeeOther)
		return
	}
	defer file.Close()

	// Crea directory uploads/rapporti se non esiste
	uploadDir := filepath.Join("web", "static", "uploads", "rapporti")
	os.MkdirAll(uploadDir, 0755)

	// Genera nome file unico
	ext := filepath.Ext(header.Filename)
	fileName := fmt.Sprintf("%d_%d%s", rapportoID, time.Now().UnixNano(), ext)
	filePath := filepath.Join(uploadDir, fileName)

	// Salva file
	dst, err := os.Create(filePath)
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/rapporti/dettaglio/%d?error=save", rapportoID), http.StatusSeeOther)
		return
	}
	defer dst.Close()

	io.Copy(dst, file)

	// Salva riferimento nel database
	webPath := "/static/uploads/rapporti/" + fileName
	database.DB.Exec(`INSERT INTO foto_rapporto (rapporto_id, file_path, descrizione) VALUES (?, ?, ?)`,
		rapportoID, webPath, descrizione)

	http.Redirect(w, r, fmt.Sprintf("/rapporti/dettaglio/%d", rapportoID), http.StatusSeeOther)
}

// EliminaFotoRapporto elimina foto
func EliminaFotoRapporto(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/rapporti/foto/elimina/")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	// Recupera info foto
	var rapportoID int64
	var filePath string
	database.DB.QueryRow("SELECT rapporto_id, file_path FROM foto_rapporto WHERE id = ?", id).
		Scan(&rapportoID, &filePath)

	// Elimina file fisico
	realPath := strings.TrimPrefix(filePath, "/static/")
	realPath = filepath.Join("web", "static", realPath)
	os.Remove(realPath)

	// Elimina da database
	database.DB.Exec("DELETE FROM foto_rapporto WHERE id = ?", id)

	http.Redirect(w, r, fmt.Sprintf("/rapporti/dettaglio/%d", rapportoID), http.StatusSeeOther)
}

// Funzioni helper

func getRapportoFormData(rapportoID int64) map[string]interface{} {
	data := make(map[string]interface{})

	// Lista navi
	navi, _ := getNaviList()
	data["Navi"] = navi

	// Lista porti
	porti, _ := getPortiList()
	data["Porti"] = porti

	// Lista tecnici
	tecnici := getAllTecniciForSelect(rapportoID)
	data["Tecnici"] = tecnici

	// Rapporto esistente
	if rapportoID > 0 {
		var rap RapportoIntervento
		var ddtGen int
		database.DB.QueryRow(`
			SELECT id, nave_id, porto_id, tipo, data_intervento,
			       COALESCE(descrizione, ''), COALESCE(note, ''), ddt_generato, COALESCE(numero_ddt, '')
			FROM rapporti_intervento WHERE id = ?
		`, rapportoID).Scan(&rap.ID, &rap.NaveID, &rap.PortoID, &rap.Tipo, &rap.DataIntervento,
			&rap.Descrizione, &rap.Note, &ddtGen, &rap.NumeroDDT)
		rap.DDTGenerato = ddtGen == 1
		data["Rapporto"] = rap
	} else {
		data["Rapporto"] = RapportoIntervento{
			DataIntervento: time.Now().Format("2006-01-02"),
		}
	}

	return data
}

func getAllTecniciForSelect(rapportoID int64) []TecnicoRapporto {
	var tecnici []TecnicoRapporto

	rows, err := database.DB.Query(`
		SELECT id, nome, cognome FROM utenti 
		WHERE ruolo = 'tecnico' AND deleted_at IS NULL
		ORDER BY cognome, nome
	`)
	if err != nil {
		return tecnici
	}
	defer rows.Close()

	// Tecnici già assegnati
	assigned := make(map[int64]bool)
	if rapportoID > 0 {
		rows2, _ := database.DB.Query("SELECT tecnico_id FROM tecnici_rapporto WHERE rapporto_id = ?", rapportoID)
		if rows2 != nil {
			defer rows2.Close()
			for rows2.Next() {
				var tid int64
				rows2.Scan(&tid)
				assigned[tid] = true
			}
		}
	}

	for rows.Next() {
		var t TecnicoRapporto
		rows.Scan(&t.ID, &t.Nome, &t.Cognome)
		t.Selected = assigned[t.ID]
		tecnici = append(tecnici, t)
	}

	return tecnici
}

func getTecniciRapporto(rapportoID int64) []TecnicoRapporto {
	var tecnici []TecnicoRapporto

	rows, err := database.DB.Query(`
		SELECT u.id, u.nome, u.cognome
		FROM tecnici_rapporto tr
		JOIN utenti u ON tr.tecnico_id = u.id
		WHERE tr.rapporto_id = ?
	`, rapportoID)
	if err != nil {
		return tecnici
	}
	defer rows.Close()

	for rows.Next() {
		var t TecnicoRapporto
		rows.Scan(&t.ID, &t.Nome, &t.Cognome)
		tecnici = append(tecnici, t)
	}

	return tecnici
}

func getMaterialeRapporto(rapportoID int64) []MaterialeRapporto {
	var materiali []MaterialeRapporto

	rows, err := database.DB.Query(`
		SELECT mr.id, mr.prodotto_id, COALESCE(p.nome, ''), COALESCE(p.codice, ''),
		       mr.quantita, COALESCE(p.unita_misura, 'pz')
		FROM materiale_rapporto mr
		LEFT JOIN prodotti p ON mr.prodotto_id = p.id
		WHERE mr.rapporto_id = ?
	`, rapportoID)
	if err != nil {
		return materiali
	}
	defer rows.Close()

	for rows.Next() {
		var m MaterialeRapporto
		rows.Scan(&m.ID, &m.ProdottoID, &m.NomeProdotto, &m.Codice, &m.Quantita, &m.UnitaMisura)
		materiali = append(materiali, m)
	}

	return materiali
}

func getFotoRapporto(rapportoID int64) []FotoRapporto {
	var foto []FotoRapporto

	rows, err := database.DB.Query(`
		SELECT id, file_path, COALESCE(descrizione, ''), created_at
		FROM foto_rapporto
		WHERE rapporto_id = ?
		ORDER BY created_at DESC
	`, rapportoID)
	if err != nil {
		return foto
	}
	defer rows.Close()

	for rows.Next() {
		var f FotoRapporto
		rows.Scan(&f.ID, &f.FilePath, &f.Descrizione, &f.CreatedAt)
		foto = append(foto, f)
	}

	return foto
}

// getNaviList restituisce lista navi per select
func getNaviList() ([]map[string]interface{}, error) {
	var navi []map[string]interface{}

	rows, err := database.DB.Query(`
		SELECT n.id, n.nome, COALESCE(c.nome, '') as compagnia
		FROM navi n
		LEFT JOIN compagnie c ON n.compagnia_id = c.id
		
		ORDER BY c.nome, n.nome
	`)
	if err != nil {
		return navi, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var nome, compagnia string
		rows.Scan(&id, &nome, &compagnia)
		navi = append(navi, map[string]interface{}{
			"ID":        id,
			"Nome":      nome,
			"Compagnia": compagnia,
		})
	}

	return navi, nil
}

// getPortiList restituisce lista porti
func getPortiList() ([]map[string]interface{}, error) {
	var porti []map[string]interface{}

	rows, err := database.DB.Query("SELECT id, nome FROM porti ORDER BY nome")
	if err != nil {
		return porti, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var nome string
		rows.Scan(&id, &nome)
		porti = append(porti, map[string]interface{}{
			"ID":   id,
			"Nome": nome,
		})
	}

	return porti, nil
}

// getProdottiList restituisce lista prodotti con giacenza
func getProdottiList() ([]map[string]interface{}, error) {
	var prodotti []map[string]interface{}

	rows, err := database.DB.Query(`
		SELECT id, codice, nome, COALESCE(quantita, 0), unita_misura
		FROM prodotti 
		WHERE deleted_at IS NULL AND COALESCE(quantita, 0) > 0
		ORDER BY nome
	`)
	if err != nil {
		return prodotti, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var codice, nome, unitaMisura string
		var quantita float64
		rows.Scan(&id, &codice, &nome, &quantita, &unitaMisura)
		prodotti = append(prodotti, map[string]interface{}{
			"ID":          id,
			"Codice":      codice,
			"Nome":        nome,
			"Quantita":    quantita,
			"UnitaMisura": unitaMisura,
		})
	}

	return prodotti, nil
}

// RapportoPDF genera la pagina PDF del rapporto
func RapportoPDF(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/rapporti/pdf/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/rapporti", http.StatusSeeOther)
		return
	}

	// Carica rapporto
	var rap RapportoIntervento
	var ddtGen int
	err = database.DB.QueryRow(`
		SELECT r.id, r.nave_id, r.porto_id, r.tipo, r.data_intervento,
		       COALESCE(r.data_fine, ''), COALESCE(r.descrizione, ''), COALESCE(r.note, ''),
		       COALESCE(r.considerazioni_finali, ''),
		       r.ddt_generato, COALESCE(r.numero_ddt, ''),
		       COALESCE(n.nome, ''), COALESCE(c.nome, ''), COALESCE(p.nome, '')
		FROM rapporti_intervento r
		LEFT JOIN navi n ON r.nave_id = n.id
		LEFT JOIN compagnie c ON n.compagnia_id = c.id
		LEFT JOIN porti p ON r.porto_id = p.id
		WHERE r.id = ?
	`, id).Scan(&rap.ID, &rap.NaveID, &rap.PortoID, &rap.Tipo, &rap.DataIntervento,
		&rap.DataFine, &rap.Descrizione, &rap.Note, &rap.ConsiderazioniFinali,
		&ddtGen, &rap.NumeroDDT,
		&rap.NomeNave, &rap.NomeCompagnia, &rap.NomePorto)

	if err != nil {
		http.Redirect(w, r, "/rapporti", http.StatusSeeOther)
		return
	}
	rap.DDTGenerato = ddtGen == 1

	// Carica tecnici con ore
	var tecnici []TecnicoRapporto
	rowsTec, _ := database.DB.Query(`
		SELECT u.id, u.nome, u.cognome, COALESCE(tr.ore_lavoro, 0)
		FROM tecnici_rapporto tr
		JOIN utenti u ON tr.tecnico_id = u.id
		WHERE tr.rapporto_id = ?
	`, id)
	if rowsTec != nil {
		defer rowsTec.Close()
		for rowsTec.Next() {
			var t TecnicoRapporto
			rowsTec.Scan(&t.ID, &t.Nome, &t.Cognome, &t.OreLavoro)
			tecnici = append(tecnici, t)
		}
	}

	// Carica materiale utilizzato
	var materialeUtilizzato []MaterialeRapportoNew
	rowsMU, _ := database.DB.Query(`
		SELECT id, descrizione_prodotto, quantita, COALESCE(unita, 'pz')
		FROM materiale_rapporto
		WHERE rapporto_id = ? AND tipo = 'utilizzato'
	`, id)
	if rowsMU != nil {
		defer rowsMU.Close()
		for rowsMU.Next() {
			var m MaterialeRapportoNew
			rowsMU.Scan(&m.ID, &m.DescrizioneProdotto, &m.Quantita, &m.Unita)
			m.Tipo = "utilizzato"
			materialeUtilizzato = append(materialeUtilizzato, m)
		}
	}

	// Carica materiale recuperato
	var materialeRecuperato []MaterialeRapportoNew
	rowsMR, _ := database.DB.Query(`
		SELECT id, descrizione_prodotto, quantita, COALESCE(unita, 'pz')
		FROM materiale_rapporto
		WHERE rapporto_id = ? AND tipo = 'recuperato'
	`, id)
	if rowsMR != nil {
		defer rowsMR.Close()
		for rowsMR.Next() {
			var m MaterialeRapportoNew
			rowsMR.Scan(&m.ID, &m.DescrizioneProdotto, &m.Quantita, &m.Unita)
			m.Tipo = "recuperato"
			materialeRecuperato = append(materialeRecuperato, m)
		}
	}

	// Carica foto
	foto := getFotoRapporto(id)

	// Calcola totale ore
	var totaleOre float64
	for _, t := range tecnici {
		totaleOre += t.OreLavoro
	}

	pageData := NewPageData("Rapporto Intervento #"+strconv.FormatInt(id, 10), r)
	pageData.Data = map[string]interface{}{
		"Rapporto":            rap,
		"Tecnici":             tecnici,
		"MaterialeUtilizzato": materialeUtilizzato,
		"MaterialeRecuperato": materialeRecuperato,
		"Foto":                foto,
		"TotaleOre":           totaleOre,
	}

	renderTemplate(w, "rapporto_pdf.html", pageData)
}

// EliminaRapportoDefinitivo elimina completamente il rapporto
func EliminaRapportoDefinitivo(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/rapporti/elimina-definitivo/")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	// Recupera nave_id prima di eliminare
	var naveID int64
	database.DB.QueryRow("SELECT nave_id FROM rapporti_intervento WHERE id = ?", id).Scan(&naveID)

	// Elimina foto fisicamente
	rows, _ := database.DB.Query("SELECT file_path FROM foto_rapporto WHERE rapporto_id = ?", id)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var filePath string
			rows.Scan(&filePath)
			realPath := strings.TrimPrefix(filePath, "/static/")
			realPath = filepath.Join("web", "static", realPath)
			os.Remove(realPath)
		}
	}

	// Elimina record correlati
	database.DB.Exec("DELETE FROM foto_rapporto WHERE rapporto_id = ?", id)
	database.DB.Exec("DELETE FROM materiale_rapporto WHERE rapporto_id = ?", id)
	database.DB.Exec("DELETE FROM tecnici_rapporto WHERE rapporto_id = ?", id)
	database.DB.Exec("DELETE FROM storico_interventi_nave WHERE rapporto_id = ?", id)

	// Elimina rapporto
	database.DB.Exec("DELETE FROM rapporti_intervento WHERE id = ?", id)

	http.Redirect(w, r, "/rapporti", http.StatusSeeOther)
}

// StoricoInterventiNave mostra lo storico interventi per una nave
func StoricoInterventiNave(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/navi/storico/")
	naveID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	// Carica dati nave
	var nomeNave, nomeCompagnia string
	database.DB.QueryRow(`
		SELECT n.nome, COALESCE(c.nome, '')
		FROM navi n
		LEFT JOIN compagnie c ON n.compagnia_id = c.id
		WHERE n.id = ?
	`, naveID).Scan(&nomeNave, &nomeCompagnia)

	// Carica storico interventi
	var interventi []RapportoIntervento
	rows, err := database.DB.Query(`
		SELECT r.id, r.nave_id, r.porto_id, r.tipo, r.data_intervento,
		       COALESCE(r.data_fine, ''), COALESCE(r.descrizione, ''), COALESCE(r.note, ''),
		       COALESCE(r.considerazioni_finali, ''), COALESCE(r.pdf_path, ''),
		       r.ddt_generato, COALESCE(r.numero_ddt, ''),
		       COALESCE(p.nome, '') as porto
		FROM rapporti_intervento r
		LEFT JOIN porti p ON r.porto_id = p.id
		WHERE r.nave_id = ? AND r.deleted_at IS NULL
		ORDER BY r.data_intervento DESC, r.id DESC
	`, naveID)

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var rap RapportoIntervento
			var ddtGen int
			rows.Scan(&rap.ID, &rap.NaveID, &rap.PortoID, &rap.Tipo, &rap.DataIntervento,
				&rap.DataFine, &rap.Descrizione, &rap.Note, &rap.ConsiderazioniFinali, &rap.PdfPath,
				&ddtGen, &rap.NumeroDDT, &rap.NomePorto)
			rap.DDTGenerato = ddtGen == 1
			rap.NomeNave = nomeNave
			rap.NomeCompagnia = nomeCompagnia
			interventi = append(interventi, rap)
		}
	}

	pageData := NewPageData("Storico Interventi - "+nomeNave, r)
	pageData.Data = map[string]interface{}{
		"NaveID":        naveID,
		"NomeNave":      nomeNave,
		"NomeCompagnia": nomeCompagnia,
		"Interventi":    interventi,
	}

	renderTemplate(w, "storico_interventi_nave.html", pageData)
}

// getMaterialeRapportoNew carica materiale descrittivo
func getMaterialeRapportoNew(rapportoID int64, tipo string) []MaterialeRapportoNew {
	var materiali []MaterialeRapportoNew

	rows, err := database.DB.Query(`
		SELECT id, tipo, descrizione_prodotto, quantita, COALESCE(unita, 'pz')
		FROM materiale_rapporto
		WHERE rapporto_id = ? AND tipo = ?
		ORDER BY id
	`, rapportoID, tipo)
	if err != nil {
		return materiali
	}
	defer rows.Close()

	for rows.Next() {
		var m MaterialeRapportoNew
		rows.Scan(&m.ID, &m.Tipo, &m.DescrizioneProdotto, &m.Quantita, &m.Unita)
		materiali = append(materiali, m)
	}

	return materiali
}
