package handlers

import (
	"database/sql"
	"fmt"
	"furviogest/internal/database"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ArchivioPDF rappresenta un documento PDF archiviato
type ArchivioPDF struct {
	ID            int64
	FornitoreID   int64
	Tipo          string
	Numero        string
	DataDocumento time.Time
	FilePath      string
	Note          string
	CreatedAt     time.Time
	// Campi virtuali
	NomeFornitore string
	IsAmazon      bool
}

// ListaArchivioPDF mostra la lista dei PDF archiviati
func ListaArchivioPDF(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Archivio PDF - FurvioGest", r)

	// Leggi filtri dalla query string
	fornitoreIDStr := r.URL.Query().Get("fornitore_id")
	dataDa := r.URL.Query().Get("data_da")
	dataA := r.URL.Query().Get("data_a")

	// Costruisci query con filtri
	query := `
		SELECT a.id, a.fornitore_id, a.tipo, a.numero, a.data_documento, a.file_path, a.note, a.created_at,
		       f.nome as nome_fornitore, COALESCE(f.is_amazon, 0) as is_amazon
		FROM archivio_pdf a
		LEFT JOIN fornitori f ON a.fornitore_id = f.id
		WHERE 1=1
	`
	var args []interface{}

	if fornitoreIDStr != "" {
		query += " AND a.fornitore_id = ?"
		args = append(args, fornitoreIDStr)
	}
	if dataDa != "" {
		query += " AND a.data_documento >= ?"
		args = append(args, dataDa)
	}
	if dataA != "" {
		query += " AND a.data_documento <= ?"
		args = append(args, dataA)
	}

	query += " ORDER BY a.data_documento DESC, a.id DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		data.Error = "Errore nel caricamento: " + err.Error()
		renderTemplate(w, "archivio_pdf_lista.html", data)
		return
	}
	defer rows.Close()

	var archivio []ArchivioPDF
	for rows.Next() {
		var a ArchivioPDF
		var note sql.NullString
		err := rows.Scan(&a.ID, &a.FornitoreID, &a.Tipo, &a.Numero, &a.DataDocumento, &a.FilePath, &note, &a.CreatedAt, &a.NomeFornitore, &a.IsAmazon)
		if err != nil {
			continue
		}
		if note.Valid {
			a.Note = note.String
		}
		archivio = append(archivio, a)
	}

	// Carica fornitori per il filtro
	fornitori := caricaFornitoriConAmazon()

	data.Data = map[string]interface{}{
		"Archivio":        archivio,
		"Fornitori":       fornitori,
		"FiltroFornitore": fornitoreIDStr,
		"FiltroDataDa":    dataDa,
		"FiltroDataA":     dataA,
	}
	renderTemplate(w, "archivio_pdf_lista.html", data)
}


// NuovoArchivioPDF gestisce l'upload di un nuovo PDF
func NuovoArchivioPDF(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Carica PDF - FurvioGest", r)

	fornitori := caricaFornitoriConAmazon()

	data.Data = map[string]interface{}{
		"Fornitori": fornitori,
	}

	if r.Method == http.MethodGet {
		renderTemplate(w, "archivio_pdf_form.html", data)
		return
	}

	// POST - Upload PDF
	r.ParseMultipartForm(32 << 20) // 32MB max

	fornitoreIDStr := r.FormValue("fornitore_id")
	tipo := r.FormValue("tipo")
	numero := strings.TrimSpace(r.FormValue("numero"))
	dataDocStr := r.FormValue("data_documento")
	note := strings.TrimSpace(r.FormValue("note"))

	// Validazioni
	fornitoreID, err := strconv.ParseInt(fornitoreIDStr, 10, 64)
	if err != nil || fornitoreID == 0 {
		data.Error = "Seleziona un fornitore"
		renderTemplate(w, "archivio_pdf_form.html", data)
		return
	}

	if numero == "" {
		data.Error = "Il numero documento è obbligatorio"
		renderTemplate(w, "archivio_pdf_form.html", data)
		return
	}

	dataDoc, err := time.Parse("2006-01-02", dataDocStr)
	if err != nil {
		data.Error = "Data documento non valida"
		renderTemplate(w, "archivio_pdf_form.html", data)
		return
	}

	// Gestione upload PDF
	file, header, err := r.FormFile("pdf_file")
	if err != nil {
		data.Error = "Seleziona un file PDF da caricare"
		renderTemplate(w, "archivio_pdf_form.html", data)
		return
	}
	defer file.Close()

	// Verifica estensione
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".pdf" {
		data.Error = "Il file deve essere in formato PDF"
		renderTemplate(w, "archivio_pdf_form.html", data)
		return
	}

	// Crea directory se non esiste
	uploadDir := "/home/ies/furviogest/uploads/archivio_pdf"
	os.MkdirAll(uploadDir, 0755)

	// Nome file univoco
	filename := fmt.Sprintf("%d_%s_%s%s", fornitoreID, strings.ReplaceAll(numero, "/", "-"), time.Now().Format("20060102150405"), ext)
	filename = strings.ReplaceAll(filename, " ", "_")
	fullPath := filepath.Join(uploadDir, filename)

	// Salva file
	dst, err := os.Create(fullPath)
	if err != nil {
		data.Error = "Errore nel salvataggio del file"
		renderTemplate(w, "archivio_pdf_form.html", data)
		return
	}
	defer dst.Close()
	io.Copy(dst, file)

	// Inserisci record nel database
	_, err = database.DB.Exec(`
		INSERT INTO archivio_pdf (fornitore_id, tipo, numero, data_documento, file_path, note)
		VALUES (?, ?, ?, ?, ?, ?)
	`, fornitoreID, tipo, numero, dataDoc, filename, note)
	if err != nil {
		// Rimuovi il file se l'inserimento fallisce
		os.Remove(fullPath)
		data.Error = "Errore salvataggio: " + err.Error()
		renderTemplate(w, "archivio_pdf_form.html", data)
		return
	}

	http.Redirect(w, r, "/archivio-pdf", http.StatusSeeOther)
}

// DownloadArchivioPDF permette di scaricare un PDF
func DownloadArchivioPDF(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "File non trovato", http.StatusNotFound)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Error(w, "File non trovato", http.StatusNotFound)
		return
	}

	var filePath, numero string
	err = database.DB.QueryRow(`SELECT file_path, numero FROM archivio_pdf WHERE id = ?`, id).Scan(&filePath, &numero)
	if err != nil {
		http.Error(w, "File non trovato", http.StatusNotFound)
		return
	}

	fullPath := "/home/ies/furviogest/uploads/archivio_pdf/" + filePath
	
	// Verifica che il file esista
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		http.Error(w, "File non trovato", http.StatusNotFound)
		return
	}

	// Imposta header per download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"Documento_%s.pdf\"", strings.ReplaceAll(numero, "/", "-")))
	w.Header().Set("Content-Type", "application/pdf")

	http.ServeFile(w, r, fullPath)
}

// VisualizzaArchivioPDF permette di visualizzare un PDF inline
func VisualizzaArchivioPDF(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "File non trovato", http.StatusNotFound)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Error(w, "File non trovato", http.StatusNotFound)
		return
	}

	var filePath string
	err = database.DB.QueryRow(`SELECT file_path FROM archivio_pdf WHERE id = ?`, id).Scan(&filePath)
	if err != nil {
		http.Error(w, "File non trovato", http.StatusNotFound)
		return
	}

	fullPath := "/home/ies/furviogest/uploads/archivio_pdf/" + filePath
	
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		http.Error(w, "File non trovato", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	http.ServeFile(w, r, fullPath)
}

// EliminaArchivioPDF elimina un PDF
func EliminaArchivioPDF(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/archivio-pdf", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(pathParts[3], 10, 64)
	if err != nil {
		http.Redirect(w, r, "/archivio-pdf", http.StatusSeeOther)
		return
	}

	// Recupera path file
	var filePath string
	database.DB.QueryRow(`SELECT file_path FROM archivio_pdf WHERE id = ?`, id).Scan(&filePath)

	// Elimina record
	database.DB.Exec(`DELETE FROM archivio_pdf WHERE id = ?`, id)

	// Elimina file
	if filePath != "" {
		os.Remove("/home/ies/furviogest/uploads/archivio_pdf/" + filePath)
	}

	http.Redirect(w, r, "/archivio-pdf", http.StatusSeeOther)
}
