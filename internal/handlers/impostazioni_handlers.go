package handlers

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"furviogest/internal/database"
	"furviogest/internal/middleware"
	"furviogest/internal/models"
)

// ImpostazioniAziendaHandler gestisce GET e POST per le impostazioni azienda
func ImpostazioniAziendaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		SalvaImpostazioniAzienda(w, r)
		return
	}
	ImpostazioniAzienda(w, r)
}

// ImpostazioniAzienda mostra il form delle impostazioni azienda
func ImpostazioniAzienda(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil || !session.IsTecnico() {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Recupera le impostazioni correnti
	impostazioni, err := getImpostazioniAzienda()
	if err != nil {
		http.Error(w, "Errore caricamento impostazioni", http.StatusInternalServerError)
		return
	}

	// Controlla se c'è un messaggio di successo
	successMsg := ""
	if r.URL.Query().Get("success") == "1" {
		successMsg = "Impostazioni salvate con successo!"
	}

	data := map[string]interface{}{
		"Impostazioni": impostazioni,
	}

	renderTemplate(w, "impostazioni_azienda.html", PageData{
		Title:       "Impostazioni Azienda",
		Session:     session,
		Success:     successMsg,
		Data:        data,
		CurrentYear: time.Now().Year(),
	})
}

// SalvaImpostazioniAzienda salva le impostazioni azienda
func SalvaImpostazioniAzienda(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil || !session.IsTecnico() {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/impostazioni", http.StatusSeeOther)
		return
	}

	// Parse multipart form (max 10MB per i file)
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Errore parsing form", http.StatusBadRequest)
		return
	}

	// Recupera impostazioni esistenti per i path dei file
	impostazioniAttuali, _ := getImpostazioniAzienda()

	// Gestione upload logo
	logoPath := impostazioniAttuali.LogoPath
	logoFile, logoHeader, err := r.FormFile("logo")
	if err == nil {
		defer logoFile.Close()
		logoPath, err = salvaFileAzienda(logoFile, logoHeader.Filename, "logo")
		if err != nil {
			http.Error(w, "Errore salvataggio logo", http.StatusInternalServerError)
			return
		}
	}

	// Gestione upload firma email (immagine)
	firmaPath := impostazioniAttuali.FirmaEmailPath
	firmaFile, firmaHeader, err := r.FormFile("firma_email_img")
	if err == nil {
		defer firmaFile.Close()
		firmaPath, err = salvaFileAzienda(firmaFile, firmaHeader.Filename, "firma")
		if err != nil {
			http.Error(w, "Errore salvataggio firma", http.StatusInternalServerError)
			return
		}
	}

	// Aggiorna i dati nel database
	_, err = database.DB.Exec(`
		UPDATE impostazioni_azienda SET
			ragione_sociale = ?,
			partita_iva = ?,
			codice_fiscale = ?,
			indirizzo = ?,
			cap = ?,
			citta = ?,
			provincia = ?,
			telefono = ?,
			email = ?,
			pec = ?,
			sito_web = ?,
			logo_path = ?,
			firma_email_path = ?,
			firma_email_testo = ?,
			iban = ?,
			banca = ?,
			codice_sdi = ?,
			note = ?,
			updated_at = ?
		WHERE id = 1
	`,
		r.FormValue("ragione_sociale"),
		r.FormValue("partita_iva"),
		r.FormValue("codice_fiscale"),
		r.FormValue("indirizzo"),
		r.FormValue("cap"),
		r.FormValue("citta"),
		r.FormValue("provincia"),
		r.FormValue("telefono"),
		r.FormValue("email"),
		r.FormValue("pec"),
		r.FormValue("sito_web"),
		logoPath,
		firmaPath,
		r.FormValue("firma_email_testo"),
		r.FormValue("iban"),
		r.FormValue("banca"),
		r.FormValue("codice_sdi"),
		r.FormValue("note"),
		time.Now(),
	)

	if err != nil {
		http.Error(w, "Errore salvataggio impostazioni: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/impostazioni?success=1", http.StatusSeeOther)
}

// EliminaLogo rimuove il logo aziendale
func EliminaLogo(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil || !session.IsTecnico() {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	impostazioni, _ := getImpostazioniAzienda()
	if impostazioni.LogoPath != "" {
		os.Remove(impostazioni.LogoPath)
		database.DB.Exec("UPDATE impostazioni_azienda SET logo_path = '', updated_at = ? WHERE id = 1", time.Now())
	}

	http.Redirect(w, r, "/impostazioni", http.StatusSeeOther)
}

// EliminaFirmaEmail rimuove l'immagine firma email
func EliminaFirmaEmail(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil || !session.IsTecnico() {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	impostazioni, _ := getImpostazioniAzienda()
	if impostazioni.FirmaEmailPath != "" {
		os.Remove(impostazioni.FirmaEmailPath)
		database.DB.Exec("UPDATE impostazioni_azienda SET firma_email_path = '', updated_at = ? WHERE id = 1", time.Now())
	}

	http.Redirect(w, r, "/impostazioni", http.StatusSeeOther)
}

// ServeLogoAzienda serve il file logo
func ServeLogoAzienda(w http.ResponseWriter, r *http.Request) {
	impostazioni, _ := getImpostazioniAzienda()
	if impostazioni.LogoPath == "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, impostazioni.LogoPath)
}

// ServeFirmaEmail serve il file firma email
func ServeFirmaEmail(w http.ResponseWriter, r *http.Request) {
	impostazioni, _ := getImpostazioniAzienda()
	if impostazioni.FirmaEmailPath == "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, impostazioni.FirmaEmailPath)
}

// getImpostazioniAzienda recupera le impostazioni dal database
func getImpostazioniAzienda() (*models.ImpostazioniAzienda, error) {
	var imp models.ImpostazioniAzienda
	err := database.DB.QueryRow(`
		SELECT id, ragione_sociale, partita_iva, codice_fiscale, indirizzo,
			cap, citta, provincia, telefono, email, pec, sito_web,
			logo_path, firma_email_path, firma_email_testo,
			iban, banca, codice_sdi, note, updated_at
		FROM impostazioni_azienda WHERE id = 1
	`).Scan(
		&imp.ID, &imp.RagioneSociale, &imp.PartitaIVA, &imp.CodiceFiscale, &imp.Indirizzo,
		&imp.CAP, &imp.Citta, &imp.Provincia, &imp.Telefono, &imp.Email, &imp.PEC, &imp.SitoWeb,
		&imp.LogoPath, &imp.FirmaEmailPath, &imp.FirmaEmailTesto,
		&imp.IBAN, &imp.Banca, &imp.CodiceSDI, &imp.Note, &imp.UpdatedAt,
	)
	if err != nil {
		return &models.ImpostazioniAzienda{}, err
	}
	return &imp, nil
}

// GetImpostazioniAziendaExport esporta la funzione per altri package
func GetImpostazioniAziendaExport() (*models.ImpostazioniAzienda, error) {
	return getImpostazioniAzienda()
}

// salvaFileAzienda salva un file caricato nella directory uploads/azienda
func salvaFileAzienda(file io.Reader, filename string, prefix string) (string, error) {
	// Crea directory se non esiste
	uploadDir := "data/uploads/azienda"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", err
	}

	// Genera nome file univoco
	ext := strings.ToLower(filepath.Ext(filename))
	newFilename := prefix + "_" + time.Now().Format("20060102150405") + ext
	filePath := filepath.Join(uploadDir, newFilename)

	// Crea file destinazione
	dst, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	// Copia contenuto
	_, err = io.Copy(dst, file)
	if err != nil {
		return "", err
	}

	return filePath, nil
}
