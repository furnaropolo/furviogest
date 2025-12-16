package handlers

import (
	"database/sql"
	"fmt"
	"furviogest/internal/database"
	"furviogest/internal/models"
	"github.com/xuri/excelize/v2"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// OrariNaveData contiene i dati per la pagina orari di una nave
type OrariNaveData struct {
	Nave             models.Nave
	Orari            []models.OrarioNave
	Soste            []models.SostaNave
	Porti            []models.Porto
	Disegni          []models.DisegnoNave
	Servers          []models.ServerNave
	MesiDisponibili  []string
	PortiDisponibili []string
	FiltroMese       string
	FiltroPorto      string
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

	// Leggi filtri dalla query string
	filtroMese := r.URL.Query().Get("filtro_mese")
	filtroPorto := r.URL.Query().Get("filtro_porto")

	// Carica orari con filtri e calcolo sosta
	orari, mesiDisponibili, portiDisponibili := caricaOrariConFiltri(naveID, filtroMese, filtroPorto)

	// Carica soste programmate
	oggi := time.Now().Format("2006-01-02")
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

	// Carica server nave
	var servers []models.ServerNave
	rowsServers, _ := database.DB.Query("SELECT id, nave_id, nome, indirizzo_ip, porta, protocollo, username, password, note, created_at FROM server_nave WHERE nave_id = ? ORDER BY nome", naveID)
	if rowsServers != nil {
		defer rowsServers.Close()
		for rowsServers.Next() {
			var s models.ServerNave
			rowsServers.Scan(&s.ID, &s.NaveID, &s.Nome, &s.IndirizzoIP, &s.Porta, &s.Protocollo, &s.Username, &s.Password, &s.Note, &s.CreatedAt)
			servers = append(servers, s)
		}
	}

	pageData := OrariNaveData{
		Nave:             nave,
		Orari:            orari,
		Soste:            soste,
		Porti:            porti,
		Disegni:          disegni,
		Servers:          servers,
		MesiDisponibili:  mesiDisponibili,
		PortiDisponibili: portiDisponibili,
		FiltroMese:       filtroMese,
		FiltroPorto:      filtroPorto,
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

	if r.URL.Query().Get("success") == "1" {
		imported := r.URL.Query().Get("imported")
		if imported != "" {
			data.Success = fmt.Sprintf("Import completato: %s orari importati", imported)
		} else {
			data.Success = "Upload completato con successo"
		}
		if warnings := r.URL.Query().Get("warnings"); warnings != "" {
			data.Error = "Attenzione: " + strings.Replace(warnings, "|", ", ", -1)
		}
	}

	if r.Method == http.MethodPost {
		UploadOrariFile(w, r)
		return
	}

	compagnie, _ := getCompagnieList()

	rows, _ := database.DB.Query(`
		SELECT u.id, u.compagnia_id, u.nome_file, u.data_upload, u.attivo, 
		       c.nome, ut.nome || ' ' || ut.cognome
		FROM upload_orari u
		JOIN compagnie c ON u.compagnia_id = c.id
		JOIN utenti ut ON u.caricato_da = ut.id
		WHERE u.id IN (SELECT MAX(id) FROM upload_orari GROUP BY compagnia_id)
		ORDER BY u.data_upload DESC
		
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

	if r.URL.Query().Get("success") == "1" {
		imported := r.URL.Query().Get("imported")
		if imported != "" {
			data.Success = fmt.Sprintf("Import completato: %s orari importati", imported)
		} else {
			data.Success = "Upload completato con successo"
		}
		if warnings := r.URL.Query().Get("warnings"); warnings != "" {
			data.Error = "Attenzione: " + strings.Replace(warnings, "|", ", ", -1)
		}
	}

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/orari/upload", http.StatusSeeOther)
		return
	}

	r.ParseMultipartForm(32 << 20)

	compagniaID, _ := strconv.ParseInt(r.FormValue("compagnia_id"), 10, 64)
	note := strings.TrimSpace(r.FormValue("note"))

	file, header, err := r.FormFile("file_orari")
	if err != nil {
		data.Error = "Errore nel caricamento del file"
		renderTemplate(w, "upload_orari.html", data)
		return
	}
	defer file.Close()

	uploadsDir := filepath.Join("web", "static", "uploads", "orari")
	os.MkdirAll(uploadsDir, 0755)

	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("orari_%d_%d%s", compagniaID, time.Now().UnixNano(), ext)
	destPath := filepath.Join(uploadsDir, filename)

	dst, err := os.Create(destPath)
	if err != nil {
		data.Error = "Errore nel salvataggio del file"
		renderTemplate(w, "upload_orari.html", data)
		return
	}
	defer dst.Close()
	io.Copy(dst, file)

	database.DB.Exec("UPDATE upload_orari SET attivo = 0 WHERE compagnia_id = ?", compagniaID)

	_, err = database.DB.Exec(`
		INSERT INTO upload_orari (compagnia_id, nome_file, file_path, caricato_da, note, attivo)
		VALUES (?, ?, ?, ?, ?, 1)
	`, compagniaID, header.Filename, destPath, data.Session.UserID, note)

	if err != nil {
		data.Error = "Errore nel salvataggio dei dati"
		renderTemplate(w, "upload_orari.html", data)
		return
	}

	imported, warnings, parseErr := parseOrariExcel(destPath, compagniaID)
	if parseErr != nil {
		log.Printf("[ORARI] Errore parsing: %v", parseErr)
	}

	redirectURL := fmt.Sprintf("/orari/upload?success=1&imported=%d", imported)
	if len(warnings) > 0 {
		redirectURL += "&warnings=" + strings.Join(warnings, "|")
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

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

// UploadPiantinaNave gestisce l'upload dei disegni PDF della nave
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

	var path string
	database.DB.QueryRow("SELECT path FROM disegni_nave WHERE id = ?", disegnoID).Scan(&path)
	if path != "" {
		os.Remove(filepath.Join("web", "static", path))
	}

	database.DB.Exec("DELETE FROM disegni_nave WHERE id = ?", disegnoID)
	http.Redirect(w, r, fmt.Sprintf("/navi/dettaglio/%d", naveID), http.StatusSeeOther)
}

// AggiungiServerNave aggiunge un server/VM alla nave
func AggiungiServerNave(w http.ResponseWriter, r *http.Request) {
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

	nome := strings.TrimSpace(r.FormValue("nome"))
	indirizzoIP := strings.TrimSpace(r.FormValue("indirizzo_ip"))
	porta, _ := strconv.Atoi(r.FormValue("porta"))
	if porta == 0 {
		porta = 443
	}
	protocollo := r.FormValue("protocollo")
	if protocollo == "" {
		protocollo = "https"
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := strings.TrimSpace(r.FormValue("password"))
	note := strings.TrimSpace(r.FormValue("note"))

	database.DB.Exec("INSERT INTO server_nave (nave_id, nome, indirizzo_ip, porta, protocollo, username, password, note) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		naveID, nome, indirizzoIP, porta, protocollo, username, password, note)

	http.Redirect(w, r, fmt.Sprintf("/navi/dettaglio/%d", naveID), http.StatusSeeOther)
}

// EliminaServerNave elimina un server
func EliminaServerNave(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	naveID, _ := strconv.ParseInt(pathParts[3], 10, 64)
	serverID, _ := strconv.ParseInt(pathParts[4], 10, 64)

	database.DB.Exec("DELETE FROM server_nave WHERE id = ? AND nave_id = ?", serverID, naveID)

	http.Redirect(w, r, fmt.Sprintf("/navi/dettaglio/%d", naveID), http.StatusSeeOther)
}

// ModificaServerNave modifica un server esistente
func ModificaServerNave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	naveID, _ := strconv.ParseInt(pathParts[3], 10, 64)
	serverID, _ := strconv.ParseInt(pathParts[4], 10, 64)

	nome := strings.TrimSpace(r.FormValue("nome"))
	indirizzoIP := strings.TrimSpace(r.FormValue("indirizzo_ip"))
	porta, _ := strconv.Atoi(r.FormValue("porta"))
	if porta == 0 {
		porta = 443
	}
	protocollo := r.FormValue("protocollo")
	username := strings.TrimSpace(r.FormValue("username"))
	password := strings.TrimSpace(r.FormValue("password"))
	note := strings.TrimSpace(r.FormValue("note"))

	database.DB.Exec("UPDATE server_nave SET nome=?, indirizzo_ip=?, porta=?, protocollo=?, username=?, password=?, note=? WHERE id=? AND nave_id=?",
		nome, indirizzoIP, porta, protocollo, username, password, note, serverID, naveID)

	http.Redirect(w, r, fmt.Sprintf("/navi/dettaglio/%d", naveID), http.StatusSeeOther)
}

// parseOrariExcel legge il file Excel e importa gli orari
func parseOrariExcel(filePath string, compagniaID int64) (int, []string, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return 0, nil, fmt.Errorf("errore apertura file: %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows("DAYBYDAY")
	if err != nil {
		return 0, nil, fmt.Errorf("foglio DAYBYDAY non trovato: %v", err)
	}

	sigleMap := make(map[string]int64)
	naviRows, _ := database.DB.Query("SELECT id, UPPER(sigla) FROM navi WHERE compagnia_id = ? AND sigla IS NOT NULL AND sigla != ''", compagniaID)
	if naviRows != nil {
		defer naviRows.Close()
		for naviRows.Next() {
			var naveID int64
			var sigla string
			naviRows.Scan(&naveID, &sigla)
			sigleMap[sigla] = naveID
		}
	}

	database.DB.Exec("DELETE FROM orari_navi WHERE nave_id IN (SELECT id FROM navi WHERE compagnia_id = ?)", compagniaID)

	var imported int
	var warnings []string
	sigleNonTrovate := make(map[string]bool)

	for i, row := range rows {
		if i == 0 || len(row) < 6 {
			continue
		}

		dataStr := row[1]
		tratta := row[2]
		oraPartenza := row[3]
		oraArrivo := row[4]
		sigla := strings.ToUpper(strings.TrimSpace(row[5]))

		stato := ""
		if len(row) > 6 {
			stato = row[6]
		}

		if strings.Contains(stato, "NonCom") {
			continue
		}

		naveID, found := sigleMap[sigla]
		if !found {
			sigleNonTrovate[sigla] = true
			continue
		}

		var data string
		if strings.Contains(dataStr, "-") {
			parts := strings.Split(dataStr, " ")
			data = parts[0]
		} else if strings.Contains(dataStr, "/") {
			parts := strings.Split(dataStr, " ")
			datePart := parts[0]
			dateParts := strings.Split(datePart, "/")
			if len(dateParts) == 3 {
				month := dateParts[0]
				day := dateParts[1]
				year := dateParts[2]
				if len(month) == 1 {
					month = "0" + month
				}
				if len(day) == 1 {
					day = "0" + day
				}
				if len(year) == 2 {
					yearNum, _ := strconv.Atoi(year)
					if yearNum < 50 {
						year = "20" + year
					} else {
						year = "19" + year
					}
				}
				data = year + "-" + month + "-" + day
			} else {
				data = dataStr
			}
		} else {
			data = dataStr
		}

		portoPartenza := ""
		portoArrivo := ""
		if strings.Contains(tratta, " - ") {
			parti := strings.SplitN(tratta, " - ", 2)
			portoPartenza = strings.TrimSpace(parti[0])
			portoArrivo = strings.TrimSpace(parti[1])
		} else {
			portoPartenza = tratta
		}

		oraPartenza = normalizzaOra(oraPartenza)
		oraArrivo = normalizzaOra(oraArrivo)

		_, err := database.DB.Exec("INSERT INTO orari_navi (nave_id, data, porto_partenza_nome, porto_arrivo_nome, ora_partenza, ora_arrivo, fonte) VALUES (?, ?, ?, ?, ?, ?, 'import_excel')",
			naveID, data, portoPartenza, portoArrivo, oraPartenza, oraArrivo)

		if err == nil {
			imported++
		}
	}

	for sigla := range sigleNonTrovate {
		warnings = append(warnings, fmt.Sprintf("Sigla '%s' non trovata", sigla))
	}

	return imported, warnings, nil
}

func normalizzaOra(ora string) string {
	ora = strings.TrimSpace(ora)
	parts := strings.Split(ora, ":")
	if len(parts) >= 2 {
		return parts[0] + ":" + parts[1]
	}
	return ora
}

// caricaOrariConFiltri carica gli orari con filtri e calcola la sosta
func caricaOrariConFiltri(naveID int64, filtroMese, filtroPorto string) ([]models.OrarioNave, []string, []string) {
	var orari []models.OrarioNave
	mesiMap := make(map[string]bool)
	portiMap := make(map[string]bool)

	// Prima query per ottenere tutti i mesi e porti disponibili (senza filtri)
	allRows, err := database.DB.Query("SELECT data, porto_partenza_nome, porto_arrivo_nome FROM orari_navi WHERE nave_id = ?", naveID)
	if err == nil {
		defer allRows.Close()
		for allRows.Next() {
			var dataT time.Time
			var pp, pa sql.NullString
			allRows.Scan(&dataT, &pp, &pa)
			mesiMap[dataT.Format("2006-01")] = true
			if pp.Valid && pp.String != "" {
				portiMap[pp.String] = true
			}
			if pa.Valid && pa.String != "" {
				portiMap[pa.String] = true
			}
		}
	}

	// Query con filtri
	query := "SELECT id, nave_id, data, porto_partenza_nome, porto_arrivo_nome, ora_partenza, ora_arrivo, note, fonte FROM orari_navi WHERE nave_id = ?"
	args := []interface{}{naveID}

	if filtroMese != "" {
		query += " AND strftime('%Y-%m', data) = ?"
		args = append(args, filtroMese)
	}

	if filtroPorto != "" {
		query += " AND (porto_partenza_nome = ? OR porto_arrivo_nome = ?)"
		args = append(args, filtroPorto, filtroPorto)
	}

	query += " ORDER BY data, ora_partenza"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return orari, nil, nil
	}
	defer rows.Close()

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

	// Calcola tempo di sosta per ogni tratta
	for i := 0; i < len(orari); i++ {
		portoArrivo := orari[i].PortoArrivoNome
		oraArrivo := orari[i].OraArrivo
		dataArrivo := orari[i].Data

		for j := i + 1; j < len(orari); j++ {
			if orari[j].PortoPartenzaNome == portoArrivo {
				sosta := calcolaSosta(dataArrivo, oraArrivo, orari[j].Data, orari[j].OraPartenza)
				if sosta != "" {
					orari[i].SostaPorto = sosta
				}
				break
			}
		}
	}

	// Converti mappe in slice ordinate
	var mesi []string
	for m := range mesiMap {
		mesi = append(mesi, m)
	}
	sort.Strings(mesi)

	var porti []string
	for p := range portiMap {
		porti = append(porti, p)
	}
	sort.Strings(porti)

	return orari, mesi, porti
}

// calcolaSosta calcola il tempo di sosta tra arrivo e partenza
func calcolaSosta(dataArr time.Time, oraArr string, dataPart time.Time, oraPart string) string {
	arrParts := strings.Split(oraArr, ":")
	partParts := strings.Split(oraPart, ":")
	if len(arrParts) < 2 || len(partParts) < 2 {
		return ""
	}

	oraA, _ := strconv.Atoi(arrParts[0])
	minA, _ := strconv.Atoi(arrParts[1])
	oraP, _ := strconv.Atoi(partParts[0])
	minP, _ := strconv.Atoi(partParts[1])

	arrivo := dataArr.Add(time.Duration(oraA)*time.Hour + time.Duration(minA)*time.Minute)
	partenza := dataPart.Add(time.Duration(oraP)*time.Hour + time.Duration(minP)*time.Minute)

	if partenza.Before(arrivo) && dataArr.Equal(dataPart) {
		partenza = partenza.Add(24 * time.Hour)
	}

	diff := partenza.Sub(arrivo)
	if diff < 0 {
		return ""
	}

	ore := int(diff.Hours())
	minuti := int(diff.Minutes()) % 60

	if ore > 48 {
		return ""
	}

	if ore > 0 {
		return fmt.Sprintf("%dh %dm", ore, minuti)
	}
	return fmt.Sprintf("%dm", minuti)
}
