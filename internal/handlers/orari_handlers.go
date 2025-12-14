package handlers

import (
	"log"
	"database/sql"
	"fmt"
	"furviogest/internal/database"
	"furviogest/internal/models"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// OrariNaveData contiene i dati per la pagina orari di una nave
type OrariNaveData struct {
	Nave     models.Nave
	Orari    []models.OrarioNave
	Soste    []models.SostaNave
	Porti    []models.Porto
	Disegni  []models.DisegnoNave
}

// DettaglioNave mostra gli orari e le soste di una nave specifica
func DettaglioNave(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Dettaglio Nave - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	naveID, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	// Carica nave
	var nave models.Nave
	var imo, emailMaster, emailDirettore, emailIspettore, note sql.NullString
	var dataInizioLavori, dataFineLavori sql.NullTime
	var portoLavoriID sql.NullInt64
	var nomePortoLavori sql.NullString

	err = database.DB.QueryRow(`
		SELECT n.id, n.compagnia_id, n.nome, n.imo, n.email_master, n.email_direttore_macchina,
		       n.email_ispettore, n.note, n.ferma_per_lavori, n.data_inizio_lavori, 
		       n.data_fine_lavori_prevista, n.porto_lavori_id, c.nome, p.nome, COALESCE(n.piantina_path, '')
		FROM navi n
		JOIN compagnie c ON n.compagnia_id = c.id
		LEFT JOIN porti p ON n.porto_lavori_id = p.id
		WHERE n.id = ?
	`, naveID).Scan(&nave.ID, &nave.CompagniaID, &nave.Nome, &imo, &emailMaster, &emailDirettore,
		&emailIspettore, &note, &nave.FermaPerLavori, &dataInizioLavori, &dataFineLavori,
		&portoLavoriID, &nave.NomeCompagnia, &nomePortoLavori, &nave.PiantinaPath)

	if err != nil {
		data.Error = "Nave non trovata"
		renderTemplate(w, "navi_lista.html", data)
		return
	}

	if imo.Valid {
		nave.IMO = imo.String
	}
	if emailMaster.Valid {
		nave.EmailMaster = emailMaster.String
	}
	if emailDirettore.Valid {
		nave.EmailDirettoreMacchina = emailDirettore.String
	}
	if emailIspettore.Valid {
		nave.EmailIspettore = emailIspettore.String
	}
	if note.Valid {
		nave.Note = note.String
	}
	if dataInizioLavori.Valid {
		nave.DataInizioLavori = &dataInizioLavori.Time
	}
	if dataFineLavori.Valid {
		nave.DataFineLavoriPrev = &dataFineLavori.Time
	}
	if portoLavoriID.Valid {
		nave.PortoLavoriID = &portoLavoriID.Int64
	}
	if nomePortoLavori.Valid {
		nave.NomePortoLavori = nomePortoLavori.String
	}

	// Carica orari prossimi 30 giorni
	oggi := time.Now().Format("2006-01-02")
	rows, _ := database.DB.Query(`
		SELECT id, nave_id, data, porto_partenza_nome, porto_arrivo_nome, 
		       ora_partenza, ora_arrivo, note, fonte
		FROM orari_navi 
		WHERE nave_id = ? AND data >= ?
		ORDER BY data, ora_partenza
		LIMIT 50
	`, naveID, oggi)
	defer rows.Close()

	var orari []models.OrarioNave
	for rows.Next() {
		var o models.OrarioNave
		var portoPartenza, portoArrivo, oraPartenza, oraArrivo, noteOrario, fonte sql.NullString
		rows.Scan(&o.ID, &o.NaveID, &o.Data, &portoPartenza, &portoArrivo, 
			&oraPartenza, &oraArrivo, &noteOrario, &fonte)
		if portoPartenza.Valid {
			o.PortoPartenzaNome = portoPartenza.String
		}
		if portoArrivo.Valid {
			o.PortoArrivoNome = portoArrivo.String
		}
		if oraPartenza.Valid {
			o.OraPartenza = oraPartenza.String
		}
		if oraArrivo.Valid {
			o.OraArrivo = oraArrivo.String
		}
		if noteOrario.Valid {
			o.Note = noteOrario.String
		}
		if fonte.Valid {
			o.Fonte = fonte.String
		}
		orari = append(orari, o)
	}

	// Carica soste programmate
	rowsSoste, _ := database.DB.Query(`
		SELECT s.id, s.nave_id, s.porto_nome, s.data_inizio, s.data_fine,
		       s.ora_arrivo, s.ora_partenza, s.motivo, s.note, s.fonte, p.nome
		FROM soste_navi s
		LEFT JOIN porti p ON s.porto_id = p.id
		WHERE s.nave_id = ? AND (s.data_fine IS NULL OR s.data_fine >= ?)
		ORDER BY s.data_inizio
	`, naveID, oggi)
	defer rowsSoste.Close()

	var soste []models.SostaNave
	for rowsSoste.Next() {
		var s models.SostaNave
		var portoNome, oraArr, oraPart, motivo, noteSosta, fonte, nomePortoDB sql.NullString
		var dataFine sql.NullTime
		rowsSoste.Scan(&s.ID, &s.NaveID, &portoNome, &s.DataInizio, &dataFine,
			&oraArr, &oraPart, &motivo, &noteSosta, &fonte, &nomePortoDB)
		if portoNome.Valid {
			s.PortoNome = portoNome.String
		}
		if nomePortoDB.Valid {
			s.NomePorto = nomePortoDB.String
		}
		if dataFine.Valid {
			s.DataFine = &dataFine.Time
		}
		if oraArr.Valid {
			s.OraArrivo = oraArr.String
		}
		if oraPart.Valid {
			s.OraPartenza = oraPart.String
		}
		if motivo.Valid {
			s.Motivo = motivo.String
		}
		if noteSosta.Valid {
			s.Note = noteSosta.String
		}
		if fonte.Valid {
			s.Fonte = fonte.String
		}
		soste = append(soste, s)
	}

	// Carica porti per form
	porti, _ := caricaPorti()

	// Carica disegni nave
	var disegni []models.DisegnoNave
	rowsDisegni, _ := database.DB.Query("SELECT id, nave_id, nome, path, created_at FROM disegni_nave WHERE nave_id = ? ORDER BY created_at DESC", naveID)
	if rowsDisegni != nil {
		defer rowsDisegni.Close()
		for rowsDisegni.Next() {
			var d models.DisegnoNave
			rowsDisegni.Scan(&d.ID, &d.NaveID, &d.Nome, &d.Path, &d.CreatedAt)
			disegni = append(disegni, d)
		}
	}

	pageData := OrariNaveData{
		Nave:  nave,
		Orari: orari,
		Soste: soste,
		Porti:    porti,
		Disegni:  disegni,
	}

	data.Data = pageData
	renderTemplate(w, "nave_dettaglio.html", data)
}

// NuovoOrario aggiunge un orario/tratta manualmente
func NuovoOrario(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	naveID, _ := strconv.ParseInt(pathParts[4], 10, 64)

	if r.Method != http.MethodPost {
		http.Redirect(w, r, fmt.Sprintf("/navi/dettaglio/%d", naveID), http.StatusSeeOther)
		return
	}

	r.ParseForm()
	data := r.FormValue("data")
	portoPartenza := strings.TrimSpace(r.FormValue("porto_partenza"))
	portoArrivo := strings.TrimSpace(r.FormValue("porto_arrivo"))
	oraPartenza := strings.TrimSpace(r.FormValue("ora_partenza"))
	oraArrivo := strings.TrimSpace(r.FormValue("ora_arrivo"))
	note := strings.TrimSpace(r.FormValue("note"))

	_, err := database.DB.Exec(`
		INSERT INTO orari_navi (nave_id, data, porto_partenza_nome, porto_arrivo_nome, 
		                        ora_partenza, ora_arrivo, note, fonte)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'manuale')
	`, naveID, data, portoPartenza, portoArrivo, oraPartenza, oraArrivo, note)

	if err != nil {
		http.Error(w, "Errore salvataggio", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/navi/dettaglio/%d", naveID), http.StatusSeeOther)
}

// EliminaOrario rimuove un orario
func EliminaOrario(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	orarioID, _ := strconv.ParseInt(pathParts[4], 10, 64)

	// Recupera nave_id prima di eliminare
	var naveID int64
	database.DB.QueryRow("SELECT nave_id FROM orari_navi WHERE id = ?", orarioID).Scan(&naveID)

	database.DB.Exec("DELETE FROM orari_navi WHERE id = ?", orarioID)

	http.Redirect(w, r, fmt.Sprintf("/navi/dettaglio/%d", naveID), http.StatusSeeOther)
}

// NuovaSosta aggiunge una sosta programmata
func NuovaSosta(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	naveID, _ := strconv.ParseInt(pathParts[4], 10, 64)

	if r.Method != http.MethodPost {
		http.Redirect(w, r, fmt.Sprintf("/navi/dettaglio/%d", naveID), http.StatusSeeOther)
		return
	}

	r.ParseForm()
	
	var portoID *int64
	if pid := r.FormValue("porto_id"); pid != "" {
		id, _ := strconv.ParseInt(pid, 10, 64)
		portoID = &id
	}
	
	portoNome := strings.TrimSpace(r.FormValue("porto_nome"))
	dataInizio := r.FormValue("data_inizio")
	dataFine := r.FormValue("data_fine")
	oraArrivo := strings.TrimSpace(r.FormValue("ora_arrivo"))
	oraPartenza := strings.TrimSpace(r.FormValue("ora_partenza"))
	motivo := strings.TrimSpace(r.FormValue("motivo"))
	note := strings.TrimSpace(r.FormValue("note"))

	var dataFineVal interface{}
	if dataFine != "" {
		dataFineVal = dataFine
	}

	_, err := database.DB.Exec(`
		INSERT INTO soste_navi (nave_id, porto_id, porto_nome, data_inizio, data_fine,
		                        ora_arrivo, ora_partenza, motivo, note, fonte)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'manuale')
	`, naveID, portoID, portoNome, dataInizio, dataFineVal, oraArrivo, oraPartenza, motivo, note)

	if err != nil {
		http.Error(w, "Errore salvataggio: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/navi/dettaglio/%d", naveID), http.StatusSeeOther)
}

// EliminaSosta rimuove una sosta
func EliminaSosta(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	sostaID, _ := strconv.ParseInt(pathParts[4], 10, 64)

	var naveID int64
	database.DB.QueryRow("SELECT nave_id FROM soste_navi WHERE id = ?", sostaID).Scan(&naveID)

	database.DB.Exec("DELETE FROM soste_navi WHERE id = ?", sostaID)

	http.Redirect(w, r, fmt.Sprintf("/navi/dettaglio/%d", naveID), http.StatusSeeOther)
}

// UploadOrariPage mostra la pagina per upload file orari
func UploadOrariPage(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Upload Orari - FurvioGest", r)

	if r.Method == http.MethodPost {
		UploadOrariFile(w, r)
		return
	}

	// Carica compagnie
	compagnie, _ := getCompagnieList()

	// Carica upload recenti
	rows, _ := database.DB.Query(`
		SELECT u.id, u.compagnia_id, u.nome_file, u.data_upload, u.attivo, 
		       c.nome, ut.nome || ' ' || ut.cognome
		FROM upload_orari u
		JOIN compagnie c ON u.compagnia_id = c.id
		JOIN utenti ut ON u.caricato_da = ut.id
		ORDER BY u.data_upload DESC
		LIMIT 20
	`)
	defer rows.Close()

	var uploads []models.UploadOrari
	for rows.Next() {
		var u models.UploadOrari
		rows.Scan(&u.ID, &u.CompagniaID, &u.NomeFile, &u.DataUpload, &u.Attivo,
			&u.NomeCompagnia, &u.NomeUtente)
		uploads = append(uploads, u)
	}

	pageData := map[string]interface{}{
		"Compagnie": compagnie,
		"Uploads":   uploads,
	}

	data.Data = pageData
	renderTemplate(w, "upload_orari.html", data)
}

// UploadOrariFile gestisce l'upload del file XLSX
func UploadOrariFile(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Upload Orari - FurvioGest", r)

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/orari/upload", http.StatusSeeOther)
		return
	}

	r.ParseMultipartForm(32 << 20) // 32MB max

	compagniaID, _ := strconv.ParseInt(r.FormValue("compagnia_id"), 10, 64)
	note := strings.TrimSpace(r.FormValue("note"))

	file, header, err := r.FormFile("file_orari")
	if err != nil {
		data.Error = "Errore nel caricamento del file"
		renderTemplate(w, "upload_orari.html", data)
		return
	}
	defer file.Close()

	// Crea directory uploads se non esiste
	uploadsDir := filepath.Join("web", "static", "uploads", "orari")
	os.MkdirAll(uploadsDir, 0755)

	// Genera nome file unico
	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("orari_%d_%d%s", compagniaID, time.Now().UnixNano(), ext)
	destPath := filepath.Join(uploadsDir, filename)

	// Salva file
	dst, err := os.Create(destPath)
	if err != nil {
		data.Error = "Errore nel salvataggio del file"
		renderTemplate(w, "upload_orari.html", data)
		return
	}
	defer dst.Close()
	io.Copy(dst, file)

	// Disattiva upload precedenti della stessa compagnia
	database.DB.Exec("UPDATE upload_orari SET attivo = 0 WHERE compagnia_id = ?", compagniaID)

	// Registra nuovo upload
	_, err = database.DB.Exec(`
		INSERT INTO upload_orari (compagnia_id, nome_file, file_path, caricato_da, note, attivo)
		VALUES (?, ?, ?, ?, ?, 1)
	`, compagniaID, header.Filename, destPath, data.Session.UserID, note)

	if err != nil {
		data.Error = "Errore nel salvataggio dei dati"
		renderTemplate(w, "upload_orari.html", data)
		return
	}

	// TODO: Parsing del file XLSX per estrarre gli orari
	// Per ora salviamo solo il file, il parsing può essere implementato dopo

	http.Redirect(w, r, "/orari/upload?success=1", http.StatusSeeOther)
}

// Funzione helper per ottenere lista compagnie
func getCompagnieList() ([]models.Compagnia, error) {
	rows, err := database.DB.Query("SELECT id, nome FROM compagnie ORDER BY nome")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var compagnie []models.Compagnia
	for rows.Next() {
		var c models.Compagnia
		rows.Scan(&c.ID, &c.Nome)
		compagnie = append(compagnie, c)
	}
	return compagnie, nil
}

// UploadDisegnoNave gestisce l upload dei disegni PDF della nave
func UploadPiantinaNave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	naveID, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	err = r.ParseMultipartForm(50 << 20)
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/navi/dettaglio/%d", naveID), http.StatusSeeOther)
		return
	}

	nomeDisegno := strings.TrimSpace(r.FormValue("nome_disegno"))
	if nomeDisegno == "" {
		nomeDisegno = "Disegno"
	}

	file, header, err := r.FormFile("disegno")
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/navi/dettaglio/%d", naveID), http.StatusSeeOther)
		return
	}
	defer file.Close()

	uploadDir := filepath.Join("web", "static", "uploads", "disegni")
	os.MkdirAll(uploadDir, 0755)

	ext := filepath.Ext(header.Filename)
	newFilename := fmt.Sprintf("disegno_nave_%d_%d%s", naveID, time.Now().Unix(), ext)
	filePath := filepath.Join(uploadDir, newFilename)

	dst, err := os.Create(filePath)
	if err != nil {
		log.Println("Errore creazione file disegno:", err)
		http.Redirect(w, r, fmt.Sprintf("/navi/dettaglio/%d", naveID), http.StatusSeeOther)
		return
	}
	defer dst.Close()
	io.Copy(dst, file)

	disegnoPath := filepath.Join("uploads", "disegni", newFilename)
	database.DB.Exec("INSERT INTO disegni_nave (nave_id, nome, path) VALUES (?, ?, ?)", naveID, nomeDisegno, disegnoPath)

	http.Redirect(w, r, fmt.Sprintf("/navi/dettaglio/%d", naveID), http.StatusSeeOther)
}

// EliminaDisegnoNave elimina un disegno
func EliminaDisegnoNave(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	naveID, _ := strconv.ParseInt(pathParts[3], 10, 64)
	disegnoID, _ := strconv.ParseInt(pathParts[4], 10, 64)

	// Recupera path per eliminare file
	var path string
	database.DB.QueryRow("SELECT path FROM disegni_nave WHERE id = ?", disegnoID).Scan(&path)
	if path != "" {
		os.Remove(filepath.Join("web", "static", path))
	}

	database.DB.Exec("DELETE FROM disegni_nave WHERE id = ?", disegnoID)
	http.Redirect(w, r, fmt.Sprintf("/navi/dettaglio/%d", naveID), http.StatusSeeOther)
}
