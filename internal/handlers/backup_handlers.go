package handlers

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"furviogest/internal/database"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// BackupConfig rappresenta la configurazione del sistema di backup
type BackupConfig struct {
	ID            int64
	NasAbilitato  bool
	NasPath       string
	NasUsername   string
	NasPassword   string
	RetentionDays int
	OraBackup     string
	UpdatedAt          time.Time
	NasConfigAbilitato bool
	NasConfigRetention int
}

// BackupLog rappresenta un log di backup
type BackupLog struct {
	ID        int64
	Filename  string
	Tipo      string
	Dimensione int64
	LocaleOK  bool
	NasOK     bool
	Errore    string
	CreatedAt time.Time
}

// BackupInfo rappresenta le informazioni su un file di backup
type BackupInfo struct {
	Filename   string
	Dimensione int64
	DataOra    time.Time
	Tipo       string
}

const (
	backupDir = "/home/ies/furviogest/backups"
	dataDir   = "/home/ies/furviogest/data"
)

// ============================================
// PAGINA BACKUP
// ============================================

func BackupPage(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Backup e Ripristino - FurvioGest", r)

	// Carica configurazione
	config := getBackupConfig()

	// Carica lista backup disponibili
	backups := getBackupList()

	// Carica ultimo log backup
	var ultimoBackup BackupLog
	database.DB.QueryRow(`
		SELECT id, filename, tipo, dimensione, locale_ok, nas_ok, COALESCE(errore,''), created_at
		FROM backup_sistema_log
		ORDER BY created_at DESC LIMIT 1
	`).Scan(&ultimoBackup.ID, &ultimoBackup.Filename, &ultimoBackup.Tipo,
		&ultimoBackup.Dimensione, &ultimoBackup.LocaleOK, &ultimoBackup.NasOK,
		&ultimoBackup.Errore, &ultimoBackup.CreatedAt)

	// Verifica errore backup per mostrare alert
	erroreBackup := ""
	if ultimoBackup.ID > 0 && (!ultimoBackup.LocaleOK || (config.NasAbilitato && !ultimoBackup.NasOK)) {
		erroreBackup = ultimoBackup.Errore
	}

	data.Data = map[string]interface{}{
		"Config":        config,
		"Backups":       backups,
		"UltimoBackup":  ultimoBackup,
		"ErroreBackup":  erroreBackup,
	}

	// Messaggi dalla query string
	if msg := r.URL.Query().Get("success"); msg != "" {
		switch msg {
		case "backup":
			data.Success = "Backup completato con successo"
		case "restore":
			data.Success = "Ripristino completato con successo. Il server verra riavviato."
		case "config":
			data.Success = "Configurazione salvata"
		case "nas_test":
			data.Success = "Connessione NAS riuscita"
		case "nas_saved":
			data.Success = "Connessione NAS testata e configurazione salvata con successo"
		case "nas_disabled":
			data.Success = "Backup su NAS disabilitato"
		}
	}
	if msg := r.URL.Query().Get("error"); msg != "" {
		switch msg {
		case "backup":
			data.Error = "Errore durante il backup"
		case "restore":
			data.Error = "Errore durante il ripristino"
		case "nas_test":
			data.Error = "Connessione NAS fallita: " + r.URL.Query().Get("detail")
		case "upload":
			data.Error = "Errore upload file"
		case "invalid":
			data.Error = "File di backup non valido"
		}
	}

	renderTemplate(w, "backup.html", data)
}

// ============================================
// ESEGUI BACKUP
// ============================================

func EseguiBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/backup", http.StatusSeeOther)
		return
	}

	err := eseguiBackupInterno("manuale")
	if err != nil {
		log.Printf("Errore backup manuale: %v", err)
		http.Redirect(w, r, "/backup?error=backup", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/backup?success=backup", http.StatusSeeOther)
}

// eseguiBackupInterno esegue il backup e ritorna errore se fallisce
func eseguiBackupInterno(tipo string) error {
	// Crea directory backup se non esiste
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("impossibile creare directory backup: %v", err)
	}

	// Nome file con timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("furviogest_%s.tar.gz", timestamp)
	filepath := filepath.Join(backupDir, filename)

	// Crea archivio tar.gz
	dimensione, err := creaArchivioBackup(filepath)
	if err != nil {
		logBackup(filename, tipo, 0, false, false, err.Error())
		return err
	}

	// Copia su NAS se abilitato
	config := getBackupConfig()
	nasOK := true
	nasErr := ""
	if config.NasAbilitato && config.NasPath != "" {
		if err := copiaSuNAS(filepath, config); err != nil {
			nasOK = false
			nasErr = err.Error()
			log.Printf("Errore copia NAS: %v", err)
		}
	}

	// Log del backup
	errLog := ""
	if !nasOK {
		errLog = "Backup locale OK, errore NAS: " + nasErr
	}
	logBackup(filename, tipo, dimensione, true, nasOK, errLog)

	// Pulizia vecchi backup
	pulisciVecchiBackup(config.RetentionDays)

	return nil
}

// creaArchivioBackup crea il file tar.gz con DB e uploads
func creaArchivioBackup(destPath string) (int64, error) {
	file, err := os.Create(destPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Aggiungi database
	dbPath := filepath.Join(dataDir, "furviogest.db")
	if err := aggiungiFileATar(tarWriter, dbPath, "furviogest.db"); err != nil {
		return 0, fmt.Errorf("errore aggiunta database: %v", err)
	}

	// Aggiungi cartella uploads se esiste
	uploadsDir := filepath.Join(dataDir, "uploads")
	if _, err := os.Stat(uploadsDir); err == nil {
		err = filepath.Walk(uploadsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			relPath, _ := filepath.Rel(dataDir, path)
			return aggiungiFileATar(tarWriter, path, relPath)
		})
		if err != nil {
			return 0, fmt.Errorf("errore aggiunta uploads: %v", err)
		}
	}

	// Chiudi tutto per ottenere dimensione corretta
	tarWriter.Close()
	gzWriter.Close()
	file.Close()

	// Ottieni dimensione file
	info, err := os.Stat(destPath)
	if err != nil {
		return 0, err
	}

	return info.Size(), nil
}

func aggiungiFileATar(tw *tar.Writer, filePath, nameInArchive string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = nameInArchive

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tw, file)
	return err
}

// ============================================
// RESTORE BACKUP
// ============================================

func RipristinaBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/backup", http.StatusSeeOther)
		return
	}

	filename := r.FormValue("filename")
	if filename == "" {
		http.Redirect(w, r, "/backup?error=invalid", http.StatusSeeOther)
		return
	}

	// Verifica che il file esista
	backupPath := filepath.Join(backupDir, filename)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		http.Redirect(w, r, "/backup?error=invalid", http.StatusSeeOther)
		return
	}

	// Esegui restore
	if err := eseguiRestore(backupPath); err != nil {
		log.Printf("Errore restore: %v", err)
		http.Redirect(w, r, "/backup?error=restore", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/backup?success=restore", http.StatusSeeOther)
}

// UploadBackup gestisce l'upload di un file di backup
func UploadBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/backup", http.StatusSeeOther)
		return
	}

	// Max 100MB
	r.ParseMultipartForm(100 << 20)

	file, header, err := r.FormFile("backup_file")
	if err != nil {
		http.Redirect(w, r, "/backup?error=upload", http.StatusSeeOther)
		return
	}
	defer file.Close()

	// Verifica estensione
	if !strings.HasSuffix(header.Filename, ".tar.gz") {
		http.Redirect(w, r, "/backup?error=invalid", http.StatusSeeOther)
		return
	}

	// Crea directory se non esiste
	os.MkdirAll(backupDir, 0755)

	// Salva file
	destPath := filepath.Join(backupDir, header.Filename)
	dest, err := os.Create(destPath)
	if err != nil {
		http.Redirect(w, r, "/backup?error=upload", http.StatusSeeOther)
		return
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		http.Redirect(w, r, "/backup?error=upload", http.StatusSeeOther)
		return
	}

	// Esegui restore
	if r.FormValue("restore_now") == "1" {
		if err := eseguiRestore(destPath); err != nil {
			log.Printf("Errore restore da upload: %v", err)
			http.Redirect(w, r, "/backup?error=restore", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/backup?success=restore", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/backup?success=backup", http.StatusSeeOther)
}

func eseguiRestore(archivePath string) error {
	// Apri archivio
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("file non valido: %v", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	// Directory temporanea per estrazione
	tempDir, err := os.MkdirTemp("", "furviogest_restore_")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	// Estrai file
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		targetPath := filepath.Join(tempDir, header.Name)

		// Crea directory parent se necessario
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		if header.Typeflag == tar.TypeReg {
			outFile, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	// Chiudi connessione database prima di sovrascrivere
	database.DB.Close()

	// Copia database
	tempDB := filepath.Join(tempDir, "furviogest.db")
	if _, err := os.Stat(tempDB); err == nil {
		destDB := filepath.Join(dataDir, "furviogest.db")
		if err := copyFile(tempDB, destDB); err != nil {
			return fmt.Errorf("errore copia database: %v", err)
		}
	}

	// Copia uploads se presenti
	tempUploads := filepath.Join(tempDir, "uploads")
	if _, err := os.Stat(tempUploads); err == nil {
		destUploads := filepath.Join(dataDir, "uploads")
		os.RemoveAll(destUploads)
		if err := copyDir(tempUploads, destUploads); err != nil {
			return fmt.Errorf("errore copia uploads: %v", err)
		}
	}

	// Riapri connessione database
	database.InitDB(filepath.Join(dataDir, "furviogest.db"))

	return nil
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	dest, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dest.Close()

	_, err = io.Copy(dest, source)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return copyFile(path, targetPath)
	})
}

// ============================================
// CONFIGURAZIONE NAS
// ============================================

func SalvaConfigBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/backup", http.StatusSeeOther)
		return
	}

	nasAbilitato := r.FormValue("nas_abilitato") == "1"
	nasPath := strings.TrimSpace(r.FormValue("nas_path"))
	nasUsername := strings.TrimSpace(r.FormValue("nas_username"))
	nasPassword := r.FormValue("nas_password")
	retentionDays, _ := strconv.Atoi(r.FormValue("retention_days"))
	if retentionDays < 1 {
		retentionDays = 7
	}

	// Se la password è vuota, mantieni quella esistente
	if nasPassword == "" {
		var oldPassword string
		database.DB.QueryRow("SELECT nas_password FROM backup_sistema_config WHERE id = 1").Scan(&oldPassword)
		nasPassword = oldPassword
	}

	_, err := database.DB.Exec(`
		UPDATE backup_sistema_config
		SET nas_abilitato = ?, nas_path = ?, nas_username = ?, nas_password = ?,
		    retention_days = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, nasAbilitato, nasPath, nasUsername, nasPassword, retentionDays)

	if err != nil {
		log.Printf("Errore salvataggio config backup: %v", err)
		http.Redirect(w, r, "/backup?error=config", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/backup?success=config", http.StatusSeeOther)
}

func TestNAS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/backup", http.StatusSeeOther)
		return
	}

	nasPath := strings.TrimSpace(r.FormValue("nas_path"))
	nasUsername := strings.TrimSpace(r.FormValue("nas_username"))
	nasPassword := r.FormValue("nas_password")

	// Se password vuota, usa quella salvata
	if nasPassword == "" {
		var savedPassword string
		database.DB.QueryRow("SELECT nas_password FROM backup_sistema_config WHERE id = 1").Scan(&savedPassword)
		nasPassword = savedPassword
	}

	err := testConnessioneNAS(nasPath, nasUsername, nasPassword)
	if err != nil {
		http.Redirect(w, r, "/backup?error=nas_test&detail="+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/backup?success=nas_test", http.StatusSeeOther)
}

// TestAndSaveNAS testa la connessione e se ok salva la configurazione
func TestAndSaveNAS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/backup", http.StatusSeeOther)
		return
	}

	nasPath := strings.TrimSpace(r.FormValue("nas_path"))
	nasUsername := strings.TrimSpace(r.FormValue("nas_username"))
	nasPassword := r.FormValue("nas_password")
	retentionDays, _ := strconv.Atoi(r.FormValue("retention_days"))
	if retentionDays < 1 {
		retentionDays = 7
	}

	// Se password vuota, usa quella salvata
	if nasPassword == "" {
		var savedPassword string
		database.DB.QueryRow("SELECT nas_password FROM backup_sistema_config WHERE id = 1").Scan(&savedPassword)
		nasPassword = savedPassword
	}

	// Prima testa la connessione
	err := testConnessioneNAS(nasPath, nasUsername, nasPassword)
	if err != nil {
		http.Redirect(w, r, "/backup?error=nas_test&detail="+err.Error(), http.StatusSeeOther)
		return
	}

	// Test OK, salva la configurazione con NAS abilitato
	_, err = database.DB.Exec(`
		UPDATE backup_sistema_config
		SET nas_abilitato = 1, nas_path = ?, nas_username = ?, nas_password = ?,
		    retention_days = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, nasPath, nasUsername, nasPassword, retentionDays)

	if err != nil {
		log.Printf("Errore salvataggio config backup: %v", err)
		http.Redirect(w, r, "/backup?error=config", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/backup?success=nas_saved", http.StatusSeeOther)
}

// DisableNAS disabilita il backup su NAS
func DisableNAS(w http.ResponseWriter, r *http.Request) {
	database.DB.Exec("UPDATE backup_sistema_config SET nas_abilitato = 0, updated_at = CURRENT_TIMESTAMP WHERE id = 1")
	http.Redirect(w, r, "/backup?success=nas_disabled", http.StatusSeeOther)
}

// parseNASPath separa share e sottocartella dal percorso NAS
// Input: //192.168.1.15/Operational/Ciccio/furvio
// Output: share=//192.168.1.15/Operational, subdir=Ciccio/furvio
func parseNASPath(fullPath string) (share string, subdir string) {
	// Rimuovi // iniziale per il parsing
	path := strings.TrimPrefix(fullPath, "//")
	parts := strings.SplitN(path, "/", 3) // server, share, resto

	if len(parts) >= 2 {
		share = "//" + parts[0] + "/" + parts[1]
	}
	if len(parts) >= 3 {
		subdir = parts[2]
	}
	return
}

func testConnessioneNAS(path, username, password string) error {
	// Usa smbclient per testare la connessione (non richiede root)
	share, subdir := parseNASPath(path)

	// Prima testa connessione alla share
	var cmd *exec.Cmd
	if subdir != "" {
		// Se c'è sottocartella, prova a listare quella
		cmd = exec.Command("smbclient", share, "-U", username+"%"+password, "-c", fmt.Sprintf("cd %s; ls", subdir))
	} else {
		cmd = exec.Command("smbclient", share, "-U", username+"%"+password, "-c", "ls")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("connessione fallita: %s", string(output))
	}

	return nil
}

func copiaSuNAS(localPath string, config BackupConfig) error {
	// Usa smbclient per copiare il file (non richiede root)
	filename := filepath.Base(localPath)
	share, subdir := parseNASPath(config.NasPath)

	var cmdStr string
	if subdir != "" {
		// cd nella sottocartella e poi put
		cmdStr = fmt.Sprintf("cd %s; put %s %s", subdir, localPath, filename)
	} else {
		cmdStr = fmt.Sprintf("put %s %s", localPath, filename)
	}

	cmd := exec.Command("smbclient", share,
		"-U", config.NasUsername+"%"+config.NasPassword,
		"-c", cmdStr)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("copia NAS fallita: %s", string(output))
	}

	// Pulizia vecchi backup su NAS
	pulisciVecchiBackupNAS(config)

	return nil
}

func pulisciVecchiBackupNAS(config BackupConfig) {
	share, subdir := parseNASPath(config.NasPath)

	// Lista file su NAS
	var cmdStr string
	if subdir != "" {
		cmdStr = fmt.Sprintf("cd %s; ls furviogest_*.tar.gz", subdir)
	} else {
		cmdStr = "ls furviogest_*.tar.gz"
	}

	cmd := exec.Command("smbclient", share,
		"-U", config.NasUsername+"%"+config.NasPassword,
		"-c", cmdStr)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return
	}

	// Parse output e trova file vecchi da eliminare
	cutoffDate := time.Now().AddDate(0, 0, -config.RetentionDays)
	lines := strings.Split(string(output), "\n")

	var filesToDelete []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "furviogest_") && strings.Contains(line, ".tar.gz") {
			// Estrai nome file (prima colonna)
			parts := strings.Fields(line)
			if len(parts) > 0 {
				filename := parts[0]
				// Estrai data dal nome: furviogest_2025-12-12_15-31-34.tar.gz
				namePart := strings.TrimPrefix(filename, "furviogest_")
				namePart = strings.TrimSuffix(namePart, ".tar.gz")
				if fileDate, err := time.Parse("2006-01-02_15-04-05", namePart); err == nil {
					if fileDate.Before(cutoffDate) {
						filesToDelete = append(filesToDelete, filename)
					}
				}
			}
		}
	}

	// Elimina file vecchi
	for _, f := range filesToDelete {
		var delCmd string
		if subdir != "" {
			delCmd = fmt.Sprintf("cd %s; del %s", subdir, f)
		} else {
			delCmd = fmt.Sprintf("del %s", f)
		}
		exec.Command("smbclient", share,
			"-U", config.NasUsername+"%"+config.NasPassword,
			"-c", delCmd).Run()
	}
}

// ============================================
// FUNZIONI HELPER
// ============================================

func getBackupConfig() BackupConfig {
	var config BackupConfig
	config.RetentionDays = 7 // default
	config.NasConfigRetention = 3 // default

	database.DB.QueryRow(`
		SELECT id, nas_abilitato, COALESCE(nas_path,''), COALESCE(nas_username,''),
		       COALESCE(nas_password,''), retention_days, COALESCE(ora_backup,'00:00'), updated_at,
		       COALESCE(nas_config_abilitato, 0), COALESCE(nas_config_retention, 3)
		FROM backup_sistema_config WHERE id = 1
	`).Scan(&config.ID, &config.NasAbilitato, &config.NasPath, &config.NasUsername,
		&config.NasPassword, &config.RetentionDays, &config.OraBackup, &config.UpdatedAt,
		&config.NasConfigAbilitato, &config.NasConfigRetention)

	return config
}

func getBackupList() []BackupInfo {
	var backups []BackupInfo

	files, err := os.ReadDir(backupDir)
	if err != nil {
		return backups
	}

	for _, f := range files {
		if strings.HasPrefix(f.Name(), "furviogest_") && strings.HasSuffix(f.Name(), ".tar.gz") {
			info, err := f.Info()
			if err != nil {
				continue
			}

			// Estrai data dal nome file
			// furviogest_2025-12-12_15-04-05.tar.gz
			namePart := strings.TrimPrefix(f.Name(), "furviogest_")
			namePart = strings.TrimSuffix(namePart, ".tar.gz")
			dataOra, _ := time.Parse("2006-01-02_15-04-05", namePart)

			// Cerca tipo nel log
			tipo := "manuale"
			database.DB.QueryRow("SELECT tipo FROM backup_sistema_log WHERE filename = ?", f.Name()).Scan(&tipo)

			backups = append(backups, BackupInfo{
				Filename:   f.Name(),
				Dimensione: info.Size(),
				DataOra:    dataOra,
				Tipo:       tipo,
			})
		}
	}

	// Ordina per data decrescente
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].DataOra.After(backups[j].DataOra)
	})

	return backups
}

func logBackup(filename, tipo string, dimensione int64, localeOK, nasOK bool, errore string) {
	database.DB.Exec(`
		INSERT INTO backup_sistema_log (filename, tipo, dimensione, locale_ok, nas_ok, errore)
		VALUES (?, ?, ?, ?, ?, ?)
	`, filename, tipo, dimensione, localeOK, nasOK, errore)
}

func pulisciVecchiBackup(retentionDays int) {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	files, err := os.ReadDir(backupDir)
	if err != nil {
		return
	}

	for _, f := range files {
		if strings.HasPrefix(f.Name(), "furviogest_") && strings.HasSuffix(f.Name(), ".tar.gz") {
			info, err := f.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoffDate) {
				os.Remove(filepath.Join(backupDir, f.Name()))
				// Rimuovi anche dal log
				database.DB.Exec("DELETE FROM backup_sistema_log WHERE filename = ?", f.Name())
			}
		}
	}
}

// ============================================
// API PER BACKUP AUTOMATICO (chiamato da cron)
// ============================================

func APIBackupAutomatico(w http.ResponseWriter, r *http.Request) {
	// Verifica che sia una richiesta locale
	if !strings.HasPrefix(r.RemoteAddr, "127.0.0.1") && !strings.HasPrefix(r.RemoteAddr, "[::1]") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	err := eseguiBackupInterno("automatico")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Backup configurazioni rete su NAS se abilitato
	config := getBackupConfig()
	if config.NasConfigAbilitato {
		go eseguiBackupConfigNAS(config)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// DownloadBackup serve un file di backup per il download
func DownloadBackup(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "File non specificato", http.StatusBadRequest)
		return
	}

	filename := pathParts[3]
	// Sicurezza: verifica che sia un nome file valido
	if !strings.HasPrefix(filename, "furviogest_") || !strings.HasSuffix(filename, ".tar.gz") {
		http.Error(w, "File non valido", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(backupDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File non trovato", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "application/gzip")
	http.ServeFile(w, r, filePath)
}

// GetUltimoBackupErrore ritorna l'errore dell'ultimo backup se presente
// Usato per mostrare il banner al login
func GetUltimoBackupErrore() string {
	config := getBackupConfig()

	var localeOK, nasOK bool
	var errore string
	var createdAt time.Time

	err := database.DB.QueryRow(`
		SELECT locale_ok, nas_ok, COALESCE(errore,''), created_at
		FROM backup_sistema_log
		ORDER BY created_at DESC LIMIT 1
	`).Scan(&localeOK, &nasOK, &errore, &createdAt)

	if err != nil {
		// Nessun backup mai eseguito
		return "Nessun backup ancora eseguito. Configurare il sistema di backup."
	}

	// Controlla se il backup è più vecchio di 25 ore (margine per backup giornaliero)
	if time.Since(createdAt) > 25*time.Hour {
		return fmt.Sprintf("Ultimo backup eseguito il %s. Verificare il sistema di backup automatico.",
			createdAt.Format("02/01/2006 15:04"))
	}

	if !localeOK {
		return "Ultimo backup locale FALLITO: " + errore
	}

	if config.NasAbilitato && !nasOK {
		return "Ultimo backup su NAS FALLITO: " + errore
	}

	return ""
}

// SalvaConfigBackupNAS salva la configurazione del backup configurazioni rete su NAS
func SalvaConfigBackupNAS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/backup", http.StatusSeeOther)
		return
	}

	r.ParseForm()
	
	nasConfigAbilitato := 0
	if r.FormValue("nas_config_abilitato") == "1" {
		nasConfigAbilitato = 1
	}
	
	nasConfigRetention, _ := strconv.Atoi(r.FormValue("nas_config_retention"))
	if nasConfigRetention < 1 {
		nasConfigRetention = 3
	}
	if nasConfigRetention > 30 {
		nasConfigRetention = 30
	}

	_, err := database.DB.Exec(`
		UPDATE backup_sistema_config 
		SET nas_config_abilitato = ?, nas_config_retention = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = 1
	`, nasConfigAbilitato, nasConfigRetention)

	if err != nil {
		log.Println("Errore salvataggio config backup NAS:", err)
	}

	http.Redirect(w, r, "/backup", http.StatusSeeOther)
}

// DisabilitaConfigBackupNAS disabilita il backup configurazioni rete su NAS
func DisabilitaConfigBackupNAS(w http.ResponseWriter, r *http.Request) {
	_, err := database.DB.Exec(`
		UPDATE backup_sistema_config 
		SET nas_config_abilitato = 0, updated_at = CURRENT_TIMESTAMP 
		WHERE id = 1
	`)

	if err != nil {
		log.Println("Errore disabilitazione config backup NAS:", err)
	}

	http.Redirect(w, r, "/backup", http.StatusSeeOther)
}

// eseguiBackupConfigNAS copia i backup delle configurazioni di rete su NAS via smbclient
func eseguiBackupConfigNAS(config BackupConfig) {
	log.Println("[BACKUP CONFIG NAS] Avvio backup configurazioni rete su NAS")

	// Directory locale dei backup configurazioni
	configBackupDir := filepath.Join(dataDir, "backups")

	// Parse NAS path per ottenere share e subdir
	share, subdir := parseNASPath(config.NasPath)

	// Trova tutte le sottodirectory
	entries, err := os.ReadDir(configBackupDir)
	if err != nil {
		log.Printf("[BACKUP CONFIG NAS] Errore lettura directory: %v", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		var nasSubdir string
		var skipBackup bool

		// Determina tipo directory e sottocartella NAS
		if strings.HasPrefix(dirName, "nave_") {
			// Estrai ID nave e verifica se ferma
			naveIDStr := strings.TrimPrefix(dirName, "nave_")
			naveID, err := strconv.ParseInt(naveIDStr, 10, 64)
			if err != nil {
				continue
			}
			
			// Salta navi ferme per lavori
			var ferma int
			database.DB.QueryRow("SELECT ferma_per_lavori FROM navi WHERE id = ?", naveID).Scan(&ferma)
			if ferma == 1 {
				log.Printf("[BACKUP CONFIG NAS] Nave %d ferma per lavori, skip", naveID)
				skipBackup = true
			}
			if subdir != "" {
				nasSubdir = subdir + "/config_navi"
			} else {
				nasSubdir = "config_navi"
			}

		} else if strings.HasPrefix(dirName, "ufficio_") {
			if subdir != "" {
				nasSubdir = subdir + "/config_uffici"
			} else {
				nasSubdir = "config_uffici"
			}

		} else if strings.HasPrefix(dirName, "sala_server_") {
			if subdir != "" {
				nasSubdir = subdir + "/config_sale_server"
			} else {
				nasSubdir = "config_sale_server"
			}

		} else {
			// Directory non riconosciuta, skip
			continue
		}

		if skipBackup {
			continue
		}

		// Directory sorgente locale
		srcDir := filepath.Join(configBackupDir, dirName)

		// Copia i file su NAS via smbclient
		files, _ := os.ReadDir(srcDir)
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			localPath := filepath.Join(srcDir, f.Name())
			
			// Crea directory sul NAS e copia file
			cmdStr := fmt.Sprintf("mkdir %s; mkdir %s/%s; cd %s/%s; put %s %s", 
				nasSubdir, nasSubdir, dirName, nasSubdir, dirName, localPath, f.Name())
			
			cmd := exec.Command("smbclient", share,
				"-U", config.NasUsername+"%"+config.NasPassword,
				"-c", cmdStr)
			cmd.Run() // Ignora errori mkdir se directory esiste già
		}

		// Applica retention via smbclient
		lsCmd := fmt.Sprintf("cd %s/%s; ls *.cfg", nasSubdir, dirName)
		cmd := exec.Command("smbclient", share,
			"-U", config.NasUsername+"%"+config.NasPassword,
			"-c", lsCmd)
		output, _ := cmd.CombinedOutput()
		
		applicaRetentionNASviaSMB(share, nasSubdir, dirName, config.NasUsername, config.NasPassword, string(output), config.NasConfigRetention)
		log.Printf("[BACKUP CONFIG NAS] Backup completato per %s", dirName)
	}

	log.Println("[BACKUP CONFIG NAS] Backup completato")
}

// applicaRetentionNASviaSMB elimina i file vecchi su NAS via smbclient
func applicaRetentionNASviaSMB(nasShare, nasSubdir, dirName, username, password, lsOutput string, retention int) {
	// Parse output e raggruppa per apparato
	lines := strings.Split(lsOutput, "\n")
	groups := make(map[string][]string)
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasSuffix(line, ".cfg") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		filename := parts[0]
		// Estrai prefisso (es. "ac_AC" o "switch_SW-CAN29-229")
		nameParts := strings.Split(filename, "_")
		if len(nameParts) >= 2 {
			prefix := nameParts[0] + "_" + nameParts[1]
			groups[prefix] = append(groups[prefix], filename)
		}
	}
	
	// Per ogni gruppo, mantieni solo gli ultimi N (ordinati alfabeticamente = cronologicamente)
	for _, fileList := range groups {
		if len(fileList) <= retention {
			continue
		}
		sort.Strings(fileList)
		// Elimina i più vecchi (primi nell'ordine alfabetico dato il formato data nel nome)
		for i := 0; i < len(fileList)-retention; i++ {
			delCmd := fmt.Sprintf("cd %s/%s; del %s", nasSubdir, dirName, fileList[i])
			exec.Command("smbclient", nasShare,
				"-U", username+"%"+password,
				"-c", delCmd).Run()
		}
	}
}

// applicaRetentionNAS mantiene solo gli ultimi N backup per ogni apparato
func applicaRetentionNAS(dir string, retention int) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	// Raggruppa file per prefisso (nome apparato)
	groups := make(map[string][]os.DirEntry)
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		// Nome file: ac_NOMEAC_2025-12-14_10-00-00.txt o switch_NOME_2025-12-14.txt
		parts := strings.Split(f.Name(), "_")
		if len(parts) >= 2 {
			prefix := parts[0] + "_" + parts[1] // es. "ac_AC-MEA-01" o "switch_SW1"
			groups[prefix] = append(groups[prefix], f)
		}
	}

	// Per ogni gruppo, mantieni solo gli ultimi N
	for _, group := range groups {
		if len(group) <= retention {
			continue
		}

		// Ordina per data modifica (più vecchi prima)
		sort.Slice(group, func(i, j int) bool {
			infoI, _ := group[i].Info()
			infoJ, _ := group[j].Info()
			return infoI.ModTime().Before(infoJ.ModTime())
		})

		// Elimina i più vecchi
		for i := 0; i < len(group)-retention; i++ {
			os.Remove(filepath.Join(dir, group[i].Name()))
		}
	}
}
