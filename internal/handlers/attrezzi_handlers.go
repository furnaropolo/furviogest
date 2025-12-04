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
	"furviogest/internal/models"
)

// Attrezzo rappresenta un attrezzo o consumabile
type Attrezzo struct {
	ID                   int
	Codice               string
	Nome                 string
	Descrizione          string
	Categoria            string
	Marca                string
	Modello              string
	NumeroSerie          string
	DataAcquisto         string
	PrezzoAcquisto       float64
	FornitoreID          *int
	FornitoreNome        string
	Stato                string
	AssegnatoA           *int
	AssegnatoNome        string
	Note                 string
	DocumentoAcquistoPath string
	CreatedAt            string
}

// MovimentoAttrezzo rappresenta un movimento di attrezzo
type MovimentoAttrezzo struct {
	ID            int
	AttrezzoID    int
	TecnicoID     int
	TecnicoNome   string
	Tipo          string
	Motivo        string
	NuovoStato    string
	DocumentoPath string
	CreatedAt     string
}

// AttrezzoFormData per i form
type AttrezzoFormData struct {
	Attrezzo  Attrezzo
	Fornitori []models.Fornitore
	Tecnici   []TecnicoInfo
}

// ListaAttrezzi mostra la lista degli attrezzi
func ListaAttrezzi(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Attrezzi e Consumabili - FurvioGest", r)

	// Filtri
	filtroCategoria := r.URL.Query().Get("categoria")
	filtroStato := r.URL.Query().Get("stato")

	query := `
		SELECT a.id, COALESCE(a.codice, ''), a.nome, COALESCE(a.descrizione, ''), a.categoria,
		       COALESCE(a.marca, ''), COALESCE(a.modello, ''), COALESCE(a.numero_serie, ''),
		       COALESCE(a.data_acquisto, ''), COALESCE(a.prezzo_acquisto, 0), a.fornitore_id,
		       COALESCE(f.nome, ''), a.stato, a.assegnato_a, COALESCE(u.nome || ' ' || u.cognome, ''),
		       COALESCE(a.note, ''), COALESCE(a.documento_acquisto_path, '')
		FROM attrezzi a
		LEFT JOIN fornitori f ON a.fornitore_id = f.id
		LEFT JOIN utenti u ON a.assegnato_a = u.id
		WHERE a.deleted_at IS NULL
	`

	var args []interface{}
	if filtroCategoria != "" {
		query += " AND a.categoria = ?"
		args = append(args, filtroCategoria)
	}
	if filtroStato != "" {
		query += " AND a.stato = ?"
		args = append(args, filtroStato)
	}

	query += " ORDER BY a.categoria, a.nome"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		data.Error = "Errore caricamento attrezzi"
		renderTemplate(w, "attrezzi_lista.html", data)
		return
	}
	defer rows.Close()

	var attrezzi []Attrezzo
	for rows.Next() {
		var a Attrezzo
		rows.Scan(&a.ID, &a.Codice, &a.Nome, &a.Descrizione, &a.Categoria,
			&a.Marca, &a.Modello, &a.NumeroSerie, &a.DataAcquisto, &a.PrezzoAcquisto,
			&a.FornitoreID, &a.FornitoreNome, &a.Stato, &a.AssegnatoA, &a.AssegnatoNome,
			&a.Note, &a.DocumentoAcquistoPath)
		attrezzi = append(attrezzi, a)
	}

	data.Data = map[string]interface{}{
		"Attrezzi":         attrezzi,
		"FiltroCategoria":  filtroCategoria,
		"FiltroStato":      filtroStato,
	}

	renderTemplate(w, "attrezzi_lista.html", data)
}

// NuovoAttrezzo crea un nuovo attrezzo
func NuovoAttrezzo(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Nuovo Attrezzo - FurvioGest", r)

	fornitori, _ := getFornitoriList()
	tecnici, _ := getTecniciList()

	formData := AttrezzoFormData{
		Fornitori: fornitori,
		Tecnici:   tecnici,
	}

	if r.Method == http.MethodGet {
		data.Data = formData
		renderTemplate(w, "attrezzo_form.html", data)
		return
	}

	// POST - salva attrezzo
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		r.ParseForm()
	}

	nome := strings.TrimSpace(r.FormValue("nome"))
	if nome == "" {
		data.Error = "Il nome è obbligatorio"
		data.Data = formData
		renderTemplate(w, "attrezzo_form.html", data)
		return
	}

	categoria := r.FormValue("categoria")
	if categoria != "attrezzo" && categoria != "consumabile" {
		data.Error = "Categoria non valida"
		data.Data = formData
		renderTemplate(w, "attrezzo_form.html", data)
		return
	}

	codice := strings.TrimSpace(r.FormValue("codice"))
	descrizione := strings.TrimSpace(r.FormValue("descrizione"))
	marca := strings.TrimSpace(r.FormValue("marca"))
	modello := strings.TrimSpace(r.FormValue("modello"))
	numeroSerie := strings.TrimSpace(r.FormValue("numero_serie"))
	dataAcquisto := r.FormValue("data_acquisto")
	prezzoStr := r.FormValue("prezzo_acquisto")
	fornitoreIDStr := r.FormValue("fornitore_id")
	note := strings.TrimSpace(r.FormValue("note"))

	prezzo, _ := strconv.ParseFloat(prezzoStr, 64)
	var fornitoreID *int
	if fid, err := strconv.Atoi(fornitoreIDStr); err == nil && fid > 0 {
		fornitoreID = &fid
	}

	// Upload documento
	var documentoPath string
	file, header, err := r.FormFile("documento")
	if err == nil && header != nil {
		defer file.Close()
		ext := strings.ToLower(filepath.Ext(header.Filename))
		if ext == ".pdf" {
			uploadsDir := "web/static/uploads/attrezzi"
			os.MkdirAll(uploadsDir, 0755)
			filename := fmt.Sprintf("att_%d%s", time.Now().Unix(), ext)
			destPath := filepath.Join(uploadsDir, filename)
			destFile, err := os.Create(destPath)
			if err == nil {
				defer destFile.Close()
				io.Copy(destFile, file)
				documentoPath = "/static/uploads/attrezzi/" + filename
			}
		}
	}

	// Inserisce attrezzo
	result, err := database.DB.Exec(`
		INSERT INTO attrezzi (codice, nome, descrizione, categoria, marca, modello, numero_serie,
		                      data_acquisto, prezzo_acquisto, fornitore_id, note, documento_acquisto_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, codice, nome, descrizione, categoria, marca, modello, numeroSerie,
		dataAcquisto, prezzo, fornitoreID, note, documentoPath)
	if err != nil {
		data.Error = "Errore durante il salvataggio"
		data.Data = formData
		renderTemplate(w, "attrezzo_form.html", data)
		return
	}

	// Registra movimento carico
	attrezzoID, _ := result.LastInsertId()
	session := middleware.GetSession(r)
	database.DB.Exec(`
		INSERT INTO movimenti_attrezzi (attrezzo_id, tecnico_id, tipo, motivo, nuovo_stato, documento_path)
		VALUES (?, ?, 'carico', 'Nuovo acquisto', 'disponibile', ?)
	`, attrezzoID, session.UserID, documentoPath)

	http.Redirect(w, r, "/attrezzi", http.StatusSeeOther)
}

// ModificaAttrezzo modifica un attrezzo esistente
func ModificaAttrezzo(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Modifica Attrezzo - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/attrezzi", http.StatusSeeOther)
		return
	}
	attrezzoID, _ := strconv.ParseInt(pathParts[3], 10, 64)

	fornitori, _ := getFornitoriList()
	tecnici, _ := getTecniciList()

	var a Attrezzo
	err := database.DB.QueryRow(`
		SELECT id, COALESCE(codice, ''), nome, COALESCE(descrizione, ''), categoria,
		       COALESCE(marca, ''), COALESCE(modello, ''), COALESCE(numero_serie, ''),
		       COALESCE(data_acquisto, ''), COALESCE(prezzo_acquisto, 0), fornitore_id,
		       stato, assegnato_a, COALESCE(note, ''), COALESCE(documento_acquisto_path, '')
		FROM attrezzi WHERE id = ? AND deleted_at IS NULL
	`, attrezzoID).Scan(&a.ID, &a.Codice, &a.Nome, &a.Descrizione, &a.Categoria,
		&a.Marca, &a.Modello, &a.NumeroSerie, &a.DataAcquisto, &a.PrezzoAcquisto,
		&a.FornitoreID, &a.Stato, &a.AssegnatoA, &a.Note, &a.DocumentoAcquistoPath)
	if err != nil {
		http.Redirect(w, r, "/attrezzi", http.StatusSeeOther)
		return
	}

	formData := AttrezzoFormData{
		Attrezzo:  a,
		Fornitori: fornitori,
		Tecnici:   tecnici,
	}

	if r.Method == http.MethodGet {
		data.Data = formData
		renderTemplate(w, "attrezzo_form.html", data)
		return
	}

	// POST - aggiorna
	r.ParseForm()
	nome := strings.TrimSpace(r.FormValue("nome"))
	if nome == "" {
		data.Error = "Il nome è obbligatorio"
		data.Data = formData
		renderTemplate(w, "attrezzo_form.html", data)
		return
	}

	codice := strings.TrimSpace(r.FormValue("codice"))
	descrizione := strings.TrimSpace(r.FormValue("descrizione"))
	categoria := r.FormValue("categoria")
	marca := strings.TrimSpace(r.FormValue("marca"))
	modello := strings.TrimSpace(r.FormValue("modello"))
	numeroSerie := strings.TrimSpace(r.FormValue("numero_serie"))
	dataAcquisto := r.FormValue("data_acquisto")
	prezzoStr := r.FormValue("prezzo_acquisto")
	fornitoreIDStr := r.FormValue("fornitore_id")
	note := strings.TrimSpace(r.FormValue("note"))

	prezzo, _ := strconv.ParseFloat(prezzoStr, 64)
	var fornitoreID *int
	if fid, err := strconv.Atoi(fornitoreIDStr); err == nil && fid > 0 {
		fornitoreID = &fid
	}

	_, err = database.DB.Exec(`
		UPDATE attrezzi SET codice = ?, nome = ?, descrizione = ?, categoria = ?, marca = ?,
		       modello = ?, numero_serie = ?, data_acquisto = ?, prezzo_acquisto = ?,
		       fornitore_id = ?, note = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, codice, nome, descrizione, categoria, marca, modello, numeroSerie,
		dataAcquisto, prezzo, fornitoreID, note, attrezzoID)
	if err != nil {
		data.Error = "Errore durante il salvataggio"
		data.Data = formData
		renderTemplate(w, "attrezzo_form.html", data)
		return
	}

	http.Redirect(w, r, "/attrezzi", http.StatusSeeOther)
}

// EliminaAttrezzo elimina un attrezzo (soft delete)
func EliminaAttrezzo(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/attrezzi", http.StatusSeeOther)
		return
	}
	attrezzoID, _ := strconv.ParseInt(pathParts[3], 10, 64)

	database.DB.Exec("UPDATE attrezzi SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", attrezzoID)
	http.Redirect(w, r, "/attrezzi", http.StatusSeeOther)
}

// MovimentoAttrezzoHandler gestisce i movimenti (assegnazione, restituzione, perso, usurato, sostituzione)
func MovimentoAttrezzoHandler(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Movimento Attrezzo - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/attrezzi", http.StatusSeeOther)
		return
	}
	attrezzoID, _ := strconv.ParseInt(pathParts[3], 10, 64)

	var a Attrezzo
	err := database.DB.QueryRow(`
		SELECT id, nome, categoria, stato, assegnato_a, COALESCE(u.nome || ' ' || u.cognome, '')
		FROM attrezzi a
		LEFT JOIN utenti u ON a.assegnato_a = u.id
		WHERE a.id = ? AND a.deleted_at IS NULL
	`, attrezzoID).Scan(&a.ID, &a.Nome, &a.Categoria, &a.Stato, &a.AssegnatoA, &a.AssegnatoNome)
	if err != nil {
		http.Redirect(w, r, "/attrezzi", http.StatusSeeOther)
		return
	}

	tecnici, _ := getTecniciList()

	// Storico movimenti
	movRows, _ := database.DB.Query(`
		SELECT m.id, m.tipo, COALESCE(m.motivo, ''), COALESCE(m.nuovo_stato, ''),
		       COALESCE(m.documento_path, ''), m.created_at, u.nome || ' ' || u.cognome
		FROM movimenti_attrezzi m
		JOIN utenti u ON m.tecnico_id = u.id
		WHERE m.attrezzo_id = ?
		ORDER BY m.created_at DESC
	`, attrezzoID)
	defer movRows.Close()

	var movimenti []MovimentoAttrezzo
	for movRows.Next() {
		var m MovimentoAttrezzo
		movRows.Scan(&m.ID, &m.Tipo, &m.Motivo, &m.NuovoStato, &m.DocumentoPath, &m.CreatedAt, &m.TecnicoNome)
		movimenti = append(movimenti, m)
	}

	if r.Method == http.MethodGet {
		data.Data = map[string]interface{}{
			"Attrezzo":  a,
			"Tecnici":   tecnici,
			"Movimenti": movimenti,
		}
		renderTemplate(w, "attrezzo_movimento.html", data)
		return
	}

	// POST - registra movimento
	r.ParseForm()
	tipoMov := r.FormValue("tipo")
	motivo := strings.TrimSpace(r.FormValue("motivo"))
	assegnatoAStr := r.FormValue("assegnato_a")

	session := middleware.GetSession(r)
	var nuovoStato string
	var nuovoAssegnato *int

	switch tipoMov {
	case "assegnazione":
		if aid, err := strconv.Atoi(assegnatoAStr); err == nil && aid > 0 {
			nuovoAssegnato = &aid
			nuovoStato = "in_uso"
		}
	case "restituzione":
		nuovoStato = "disponibile"
		nuovoAssegnato = nil
	case "perso":
		nuovoStato = "perso"
		nuovoAssegnato = nil
	case "usurato":
		nuovoStato = "usurato"
	case "sostituzione":
		nuovoStato = "dismesso"
		nuovoAssegnato = nil
	case "dismesso":
		nuovoStato = "dismesso"
		nuovoAssegnato = nil
	default:
		data.Error = "Tipo movimento non valido"
		data.Data = map[string]interface{}{
			"Attrezzo":  a,
			"Tecnici":   tecnici,
			"Movimenti": movimenti,
		}
		renderTemplate(w, "attrezzo_movimento.html", data)
		return
	}

	// Aggiorna attrezzo
	database.DB.Exec(`
		UPDATE attrezzi SET stato = ?, assegnato_a = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, nuovoStato, nuovoAssegnato, attrezzoID)

	// Registra movimento
	database.DB.Exec(`
		INSERT INTO movimenti_attrezzi (attrezzo_id, tecnico_id, tipo, motivo, nuovo_stato)
		VALUES (?, ?, ?, ?, ?)
	`, attrezzoID, session.UserID, tipoMov, motivo, nuovoStato)

	http.Redirect(w, r, "/attrezzi/movimento/"+strconv.FormatInt(attrezzoID, 10), http.StatusSeeOther)
}

// StoriocoMovimentiAttrezzo mostra lo storico di un attrezzo
func StoricoAttrezzo(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Storico Attrezzo - FurvioGest", r)

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Redirect(w, r, "/attrezzi", http.StatusSeeOther)
		return
	}
	attrezzoID, _ := strconv.ParseInt(pathParts[3], 10, 64)

	var a Attrezzo
	database.DB.QueryRow(`SELECT id, nome, categoria, stato FROM attrezzi WHERE id = ?`, attrezzoID).Scan(
		&a.ID, &a.Nome, &a.Categoria, &a.Stato)

	rows, _ := database.DB.Query(`
		SELECT m.tipo, COALESCE(m.motivo, ''), COALESCE(m.nuovo_stato, ''),
		       COALESCE(m.documento_path, ''), m.created_at, u.nome || ' ' || u.cognome
		FROM movimenti_attrezzi m
		JOIN utenti u ON m.tecnico_id = u.id
		WHERE m.attrezzo_id = ?
		ORDER BY m.created_at DESC
	`, attrezzoID)
	defer rows.Close()

	var movimenti []MovimentoAttrezzo
	for rows.Next() {
		var m MovimentoAttrezzo
		rows.Scan(&m.Tipo, &m.Motivo, &m.NuovoStato, &m.DocumentoPath, &m.CreatedAt, &m.TecnicoNome)
		movimenti = append(movimenti, m)
	}

	data.Data = map[string]interface{}{
		"Attrezzo":  a,
		"Movimenti": movimenti,
	}
	renderTemplate(w, "attrezzo_storico.html", data)
}

// Helper per lista fornitori
func getFornitoriList() ([]models.Fornitore, error) {
	rows, err := database.DB.Query("SELECT id, nome FROM fornitori WHERE deleted_at IS NULL ORDER BY nome")
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
