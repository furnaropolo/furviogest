package handlers

import (
	"database/sql"
	"encoding/json"
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

// ListaDDTEntrata mostra la lista dei DDT/Fatture in entrata
func ListaDDTEntrata(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("DDT/Fatture Entrata - FurvioGest", r)

	rows, err := database.DB.Query(`
		SELECT d.id, d.tipo, d.numero, d.data_documento, d.fornitore_id, d.pdf_path, d.note, d.created_at,
		       f.nome as nome_fornitore
		FROM ddt_entrata d
		LEFT JOIN fornitori f ON d.fornitore_id = f.id
		ORDER BY d.data_documento DESC, d.id DESC
	`)
	if err != nil {
		data.Error = "Errore nel caricamento dei DDT: " + err.Error()
		renderTemplate(w, "ddt_entrata_lista.html", data)
		return
	}
	defer rows.Close()

	var ddts []models.DDTEntrata
	for rows.Next() {
		var d models.DDTEntrata
		var pdfPath, note, nomeFornitore sql.NullString
		err := rows.Scan(&d.ID, &d.Tipo, &d.Numero, &d.DataDocumento, &d.FornitoreID, &pdfPath, &note, &d.CreatedAt, &nomeFornitore)
		if err != nil {
			continue
		}
		if pdfPath.Valid {
			d.PDFPath = pdfPath.String
		}
		if note.Valid {
			d.Note = note.String
		}
		if nomeFornitore.Valid {
			d.NomeFornitore = nomeFornitore.String
		}
		ddts = append(ddts, d)
	}

	data.Data = ddts
	renderTemplate(w, "ddt_entrata_lista.html", data)
}

// NuovoDDTEntrata gestisce la creazione di un nuovo DDT/Fattura in entrata
func NuovoDDTEntrata(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Nuovo DDT/Fattura Entrata - FurvioGest", r)

	// Carica fornitori per select
	fornitori, _ := caricaFornitori()
	// Carica prodotti esistenti per select
	prodotti, _ := caricaProdottiAttivi()

	data.Data = map[string]interface{}{
		"Fornitori": fornitori,
		"Prodotti":  prodotti,
	}

	if r.Method == http.MethodGet {
		renderTemplate(w, "ddt_entrata_form.html", data)
		return
	}

	// POST - Salva DDT
	r.ParseMultipartForm(32 << 20) // 32MB max

	tipo := r.FormValue("tipo")
	numero := strings.TrimSpace(r.FormValue("numero"))
	dataDocStr := r.FormValue("data_documento")
	fornitoreIDStr := r.FormValue("fornitore_id")
	note := strings.TrimSpace(r.FormValue("note"))

	// Validazioni
	if numero == "" {
		data.Error = "Il numero documento è obbligatorio"
		renderTemplate(w, "ddt_entrata_form.html", data)
		return
	}

	dataDoc, err := time.Parse("2006-01-02", dataDocStr)
	if err != nil {
		data.Error = "Data documento non valida"
		renderTemplate(w, "ddt_entrata_form.html", data)
		return
	}

	fornitoreID, err := strconv.ParseInt(fornitoreIDStr, 10, 64)
	if err != nil || fornitoreID == 0 {
		data.Error = "Seleziona un fornitore"
		renderTemplate(w, "ddt_entrata_form.html", data)
		return
	}

	// Gestione upload PDF
	var pdfPath string
	file, header, err := r.FormFile("pdf_file")
	if err == nil {
		defer file.Close()
		// Crea directory se non esiste
		uploadDir := "/home/ies/furviogest/uploads/ddt_entrata"
		os.MkdirAll(uploadDir, 0755)
		
		// Nome file univoco
		ext := filepath.Ext(header.Filename)
		filename := fmt.Sprintf("%d_%s_%s%s", fornitoreID, numero, time.Now().Format("20060102150405"), ext)
		filename = strings.ReplaceAll(filename, "/", "-")
		filename = strings.ReplaceAll(filename, " ", "_")
		fullPath := filepath.Join(uploadDir, filename)
		pdfPath = filename
		
		dst, err := os.Create(fullPath)
		if err == nil {
			defer dst.Close()
			io.Copy(dst, file)
		}
	}

	// Inizia transazione
	tx, err := database.DB.Begin()
	if err != nil {
		data.Error = "Errore database: " + err.Error()
		renderTemplate(w, "ddt_entrata_form.html", data)
		return
	}

	// Inserisci DDT
	result, err := tx.Exec(`
		INSERT INTO ddt_entrata (tipo, numero, data_documento, fornitore_id, pdf_path, note)
		VALUES (?, ?, ?, ?, ?, ?)
	`, tipo, numero, dataDoc, fornitoreID, pdfPath, note)
	if err != nil {
		tx.Rollback()
		data.Error = "Errore salvataggio DDT: " + err.Error()
		renderTemplate(w, "ddt_entrata_form.html", data)
		return
	}

	ddtID, _ := result.LastInsertId()

	// Processa righe prodotti
	prodottiIDs := r.Form["prodotto_id[]"]
	quantitas := r.Form["quantita[]"]
	nuoviProdotti := r.Form["nuovo_prodotto[]"] // "1" se è nuovo, "0" se esistente

	for i := 0; i < len(prodottiIDs); i++ {
		if prodottiIDs[i] == "" || prodottiIDs[i] == "0" {
			continue
		}

		prodottoID, _ := strconv.ParseInt(prodottiIDs[i], 10, 64)
		quantita := 1
		if i < len(quantitas) {
			quantita, _ = strconv.Atoi(quantitas[i])
			if quantita < 1 {
				quantita = 1
			}
		}

		prodottoCreato := false
		if i < len(nuoviProdotti) && nuoviProdotti[i] == "1" {
			prodottoCreato = true
		}

		// Inserisci riga
		_, err := tx.Exec(`
			INSERT INTO ddt_entrata_righe (ddt_entrata_id, prodotto_id, quantita, prodotto_creato_da_ddt)
			VALUES (?, ?, ?, ?)
		`, ddtID, prodottoID, quantita, prodottoCreato)
		if err != nil {
			tx.Rollback()
			data.Error = "Errore salvataggio riga: " + err.Error()
			renderTemplate(w, "ddt_entrata_form.html", data)
			return
		}

		// Aggiorna giacenza prodotto
		_, err = tx.Exec(`
			UPDATE prodotti SET giacenza = giacenza + ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, quantita, prodottoID)
		if err != nil {
			tx.Rollback()
			data.Error = "Errore aggiornamento giacenza: " + err.Error()
			renderTemplate(w, "ddt_entrata_form.html", data)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		data.Error = "Errore commit: " + err.Error()
		renderTemplate(w, "ddt_entrata_form.html", data)
		return
	}

	http.Redirect(w, r, "/ddt-entrata", http.StatusSeeOther)
}

// ModificaDDTEntrata gestisce la modifica di un DDT/Fattura in entrata
func ModificaDDTEntrata(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Modifica DDT/Fattura Entrata - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/ddt-entrata", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/ddt-entrata", http.StatusSeeOther)
		return
	}

	// Carica fornitori e prodotti per select
	fornitori, _ := caricaFornitori()
	prodotti, _ := caricaProdottiAttivi()

	if r.Method == http.MethodGet {
		// Carica DDT
		var ddt models.DDTEntrata
		var pdfPath, note sql.NullString
		err := database.DB.QueryRow(`
			SELECT id, tipo, numero, data_documento, fornitore_id, pdf_path, note
			FROM ddt_entrata WHERE id = ?
		`, id).Scan(&ddt.ID, &ddt.Tipo, &ddt.Numero, &ddt.DataDocumento, &ddt.FornitoreID, &pdfPath, &note)
		if err != nil {
			http.Redirect(w, r, "/ddt-entrata", http.StatusSeeOther)
			return
		}
		if pdfPath.Valid {
			ddt.PDFPath = pdfPath.String
		}
		if note.Valid {
			ddt.Note = note.String
		}

		// Carica righe
		rows, _ := database.DB.Query(`
			SELECT r.id, r.prodotto_id, r.quantita, r.prodotto_creato_da_ddt, p.codice, p.nome
			FROM ddt_entrata_righe r
			LEFT JOIN prodotti p ON r.prodotto_id = p.id
			WHERE r.ddt_entrata_id = ?
		`, id)
		defer rows.Close()

		for rows.Next() {
			var riga models.DDTEntrataRiga
			var codice, nome sql.NullString
			rows.Scan(&riga.ID, &riga.ProdottoID, &riga.Quantita, &riga.ProdottoCreatoDaDDT, &codice, &nome)
			if codice.Valid {
				riga.CodiceProdotto = codice.String
			}
			if nome.Valid {
				riga.NomeProdotto = nome.String
			}
			ddt.Righe = append(ddt.Righe, riga)
		}

		data.Data = map[string]interface{}{
			"DDT":       ddt,
			"Fornitori": fornitori,
			"Prodotti":  prodotti,
		}
		renderTemplate(w, "ddt_entrata_form.html", data)
		return
	}

	// POST - Aggiorna DDT
	r.ParseMultipartForm(32 << 20)

	tipo := r.FormValue("tipo")
	numero := strings.TrimSpace(r.FormValue("numero"))
	dataDocStr := r.FormValue("data_documento")
	fornitoreIDStr := r.FormValue("fornitore_id")
	note := strings.TrimSpace(r.FormValue("note"))

	dataDoc, _ := time.Parse("2006-01-02", dataDocStr)
	fornitoreID, _ := strconv.ParseInt(fornitoreIDStr, 10, 64)

	// Inizia transazione
	tx, err := database.DB.Begin()
	if err != nil {
		data.Error = "Errore database"
		data.Data = map[string]interface{}{"Fornitori": fornitori, "Prodotti": prodotti}
		renderTemplate(w, "ddt_entrata_form.html", data)
		return
	}

	// Prima ripristina le giacenze delle vecchie righe
	rows, _ := tx.Query(`SELECT prodotto_id, quantita FROM ddt_entrata_righe WHERE ddt_entrata_id = ?`, id)
	for rows.Next() {
		var prodID int64
		var qta int
		rows.Scan(&prodID, &qta)
		tx.Exec(`UPDATE prodotti SET giacenza = giacenza - ? WHERE id = ?`, qta, prodID)
	}
	rows.Close()

	// Elimina vecchie righe
	tx.Exec(`DELETE FROM ddt_entrata_righe WHERE ddt_entrata_id = ?`, id)

	// Gestione upload nuovo PDF
	var pdfPath string
	file, header, err := r.FormFile("pdf_file")
	if err == nil {
		defer file.Close()
		uploadDir := "/home/ies/furviogest/uploads/ddt_entrata"
		os.MkdirAll(uploadDir, 0755)
		ext := filepath.Ext(header.Filename)
		filename := fmt.Sprintf("%d_%s_%s%s", fornitoreID, numero, time.Now().Format("20060102150405"), ext)
		filename = strings.ReplaceAll(filename, "/", "-")
		filename = strings.ReplaceAll(filename, " ", "_")
		fullPath := filepath.Join(uploadDir, filename)
		pdfPath = filename
		dst, _ := os.Create(fullPath)
		defer dst.Close()
		io.Copy(dst, file)
	}

	// Aggiorna DDT
	if pdfPath != "" {
		tx.Exec(`UPDATE ddt_entrata SET tipo=?, numero=?, data_documento=?, fornitore_id=?, pdf_path=?, note=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
			tipo, numero, dataDoc, fornitoreID, pdfPath, note, id)
	} else {
		tx.Exec(`UPDATE ddt_entrata SET tipo=?, numero=?, data_documento=?, fornitore_id=?, note=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
			tipo, numero, dataDoc, fornitoreID, note, id)
	}

	// Inserisci nuove righe e aggiorna giacenze
	prodottiIDs := r.Form["prodotto_id[]"]
	quantitas := r.Form["quantita[]"]
	nuoviProdotti := r.Form["nuovo_prodotto[]"]

	for i := 0; i < len(prodottiIDs); i++ {
		if prodottiIDs[i] == "" || prodottiIDs[i] == "0" {
			continue
		}

		prodottoID, _ := strconv.ParseInt(prodottiIDs[i], 10, 64)
		quantita := 1
		if i < len(quantitas) {
			quantita, _ = strconv.Atoi(quantitas[i])
			if quantita < 1 {
				quantita = 1
			}
		}

		prodottoCreato := false
		if i < len(nuoviProdotti) && nuoviProdotti[i] == "1" {
			prodottoCreato = true
		}

		tx.Exec(`INSERT INTO ddt_entrata_righe (ddt_entrata_id, prodotto_id, quantita, prodotto_creato_da_ddt) VALUES (?, ?, ?, ?)`,
			id, prodottoID, quantita, prodottoCreato)
		tx.Exec(`UPDATE prodotti SET giacenza = giacenza + ? WHERE id = ?`, quantita, prodottoID)
	}

	tx.Commit()
	http.Redirect(w, r, "/ddt-entrata", http.StatusSeeOther)
}

// EliminaDDTEntrata elimina un DDT e ripristina le giacenze
func EliminaDDTEntrata(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/ddt-entrata", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/ddt-entrata", http.StatusSeeOther)
		return
	}

	tx, _ := database.DB.Begin()

	// Ripristina giacenze e eventualmente elimina prodotti creati solo da questo DDT
	rows, _ := tx.Query(`
		SELECT prodotto_id, quantita, prodotto_creato_da_ddt 
		FROM ddt_entrata_righe WHERE ddt_entrata_id = ?
	`, id)
	
	var prodottiDaEliminare []int64
	for rows.Next() {
		var prodID int64
		var qta int
		var creato bool
		rows.Scan(&prodID, &qta, &creato)
		
		// Sottrai giacenza
		tx.Exec(`UPDATE prodotti SET giacenza = giacenza - ? WHERE id = ?`, qta, prodID)
		
		// Se prodotto creato da questo DDT, verifica se è usato altrove
		if creato {
			var count int
			tx.QueryRow(`SELECT COUNT(*) FROM ddt_entrata_righe WHERE prodotto_id = ? AND ddt_entrata_id != ?`, prodID, id).Scan(&count)
			if count == 0 {
				prodottiDaEliminare = append(prodottiDaEliminare, prodID)
			}
		}
	}
	rows.Close()

	// Elimina righe
	tx.Exec(`DELETE FROM ddt_entrata_righe WHERE ddt_entrata_id = ?`, id)

	// Elimina PDF se esiste
	var pdfPath sql.NullString
	tx.QueryRow(`SELECT pdf_path FROM ddt_entrata WHERE id = ?`, id).Scan(&pdfPath)
	if pdfPath.Valid && pdfPath.String != "" {
		os.Remove("/home/ies/furviogest/uploads/ddt_entrata/" + pdfPath.String)
	}

	// Elimina DDT
	tx.Exec(`DELETE FROM ddt_entrata WHERE id = ?`, id)

	// Elimina prodotti creati solo da questo DDT
	for _, prodID := range prodottiDaEliminare {
		tx.Exec(`DELETE FROM prodotti WHERE id = ?`, prodID)
	}

	tx.Commit()
	http.Redirect(w, r, "/ddt-entrata", http.StatusSeeOther)
}

// APIInfoEliminazioneDDT restituisce info su cosa verrà eliminato
func APIInfoEliminazioneDDT(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	var info struct {
		NumeroDoc       string   `json:"numero_doc"`
		NomeFornitore   string   `json:"nome_fornitore"`
		NumRighe        int      `json:"num_righe"`
		ProdottiEliminati []string `json:"prodotti_eliminati"`
	}

	database.DB.QueryRow(`
		SELECT d.numero, f.nome 
		FROM ddt_entrata d 
		LEFT JOIN fornitori f ON d.fornitore_id = f.id 
		WHERE d.id = ?
	`, id).Scan(&info.NumeroDoc, &info.NomeFornitore)

	database.DB.QueryRow(`SELECT COUNT(*) FROM ddt_entrata_righe WHERE ddt_entrata_id = ?`, id).Scan(&info.NumRighe)

	// Prodotti che verranno eliminati
	rows, _ := database.DB.Query(`
		SELECT p.nome FROM ddt_entrata_righe r
		JOIN prodotti p ON r.prodotto_id = p.id
		WHERE r.ddt_entrata_id = ? AND r.prodotto_creato_da_ddt = 1
		AND NOT EXISTS (SELECT 1 FROM ddt_entrata_righe r2 WHERE r2.prodotto_id = r.prodotto_id AND r2.ddt_entrata_id != ?)
	`, id, id)
	defer rows.Close()
	for rows.Next() {
		var nome string
		rows.Scan(&nome)
		info.ProdottiEliminati = append(info.ProdottiEliminati, nome)
	}

	json.NewEncoder(w).Encode(info)
}

// APICreaFornitoreRapido crea un fornitore al volo e restituisce l'ID
func APICreaFornitoreRapido(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if r.Method != http.MethodPost {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Metodo non permesso"})
		return
	}

	r.ParseForm()
	nome := strings.TrimSpace(r.FormValue("nome"))
	partitaIVA := strings.TrimSpace(r.FormValue("partita_iva"))
	
	if nome == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Ragione sociale obbligatoria"})
		return
	}

	result, err := database.DB.Exec(`
		INSERT INTO fornitori (nome, partita_iva, nazione) VALUES (?, ?, 'Italia')
	`, nome, partitaIVA)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"id": id,
		"nome": nome,
	})
}

// APICreaProdottoRapido crea un prodotto al volo e restituisce l'ID
func APICreaProdottoRapido(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if r.Method != http.MethodPost {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Metodo non permesso"})
		return
	}

	r.ParseForm()
	codice := strings.ToUpper(strings.TrimSpace(r.FormValue("codice")))
	nome := strings.TrimSpace(r.FormValue("nome"))
	categoria := r.FormValue("categoria")
	tipo := r.FormValue("tipo")
	
	if codice == "" || nome == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Codice e nome sono obbligatori"})
		return
	}
	if categoria == "" {
		categoria = "materiale"
	}
	if tipo == "" {
		tipo = "wifi"
	}

	result, err := database.DB.Exec(`
		INSERT INTO prodotti (codice, nome, categoria, tipo, origine, giacenza)
		VALUES (?, ?, ?, ?, 'nuovo', 0)
	`, codice, nome, categoria, tipo)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"id": id,
		"codice": codice,
		"nome": nome,
	})
}

// Helper per caricare fornitori
func caricaFornitori() ([]models.Fornitore, error) {
	rows, err := database.DB.Query(`SELECT id, nome FROM fornitori ORDER BY nome`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fornitori []models.Fornitore
	for rows.Next() {
		var f models.Fornitore
		rows.Scan(&f.ID, &f.Nome)
		fornitori = append(fornitori, f)
	}
	return fornitori, nil
}

// Helper per caricare prodotti attivi (origine = nuovo)
func caricaProdottiAttivi() ([]models.Prodotto, error) {
	rows, err := database.DB.Query(`SELECT id, codice, nome, categoria, tipo FROM prodotti WHERE origine = 'nuovo' ORDER BY nome`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prodotti []models.Prodotto
	for rows.Next() {
		var p models.Prodotto
		rows.Scan(&p.ID, &p.Codice, &p.Nome, &p.Categoria, &p.Tipo)
		prodotti = append(prodotti, p)
	}
	return prodotti, nil
}
