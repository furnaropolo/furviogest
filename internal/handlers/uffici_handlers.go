package handlers

import (
	"database/sql"
	"fmt"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"time"
	"log"
	"net/http"
	"strconv"
	"strings"

	"furviogest/internal/database"
)

// ============================================
// STRUTTURE DATI
// ============================================

type Ufficio struct {
	ID        int64
	Nome      string
	Indirizzo string
	Citta     string
	CAP       string
	Telefono  string
	Email     string
	Note      string
}

type ACUfficio struct {
	ID           int64
	UfficioID    int64
	IP           string
	SSHPort      int
	SSHUser      string
	SSHPass      string
	Protocollo   string
	Modello      string
	Note         string
	UltimoBackup string
}

type SwitchUfficio struct {
	ID           int64
	UfficioID    int64
	Nome         string
	Marca        string
	Modello      string
	IP           string
	SSHPort      int
	SSHUser      string
	SSHPass      string
	Protocollo   string
	Note         string
	UltimoBackup string
}

type UfficioPageData struct {
	Ufficio  Ufficio
	AC       *ACUfficio
	Switches []SwitchUfficio
	Backups  []ConfigBackup
}

// ============================================
// HANDLERS LISTA UFFICI
// ============================================

func ListaUffici(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Uffici - FurvioGest", r)

	rows, err := database.DB.Query("SELECT id, nome, COALESCE(indirizzo, ''), COALESCE(citta, ''), COALESCE(telefono, ''), COALESCE(email, '') FROM uffici ORDER BY nome")
	if err != nil {
		http.Error(w, "Errore caricamento uffici", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var uffici []Ufficio
	for rows.Next() {
		var u Ufficio
		rows.Scan(&u.ID, &u.Nome, &u.Indirizzo, &u.Citta, &u.Telefono, &u.Email)
		uffici = append(uffici, u)
	}

	data.Data = uffici
	renderTemplate(w, "uffici.html", data)
}

func NuovoUfficio(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		data := NewPageData("Nuovo Ufficio - FurvioGest", r)
		data.Data = Ufficio{}
		renderTemplate(w, "ufficio_form.html", data)
		return
	}

	r.ParseForm()
	nome := strings.TrimSpace(r.FormValue("nome"))
	indirizzo := strings.TrimSpace(r.FormValue("indirizzo"))
	citta := strings.TrimSpace(r.FormValue("citta"))
	cap := strings.TrimSpace(r.FormValue("cap"))
	telefono := strings.TrimSpace(r.FormValue("telefono"))
	email := strings.TrimSpace(r.FormValue("email"))
	note := strings.TrimSpace(r.FormValue("note"))

	_, err := database.DB.Exec("INSERT INTO uffici (nome, indirizzo, citta, cap, telefono, email, note) VALUES (?, ?, ?, ?, ?, ?, ?)",
		nome, indirizzo, citta, cap, telefono, email, note)
	if err != nil {
		http.Error(w, "Errore salvataggio: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/uffici", http.StatusSeeOther)
}

func ModificaUfficio(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/uffici/modifica/")
	id, _ := strconv.ParseInt(path, 10, 64)

	if r.Method == http.MethodGet {
		var u Ufficio
		err := database.DB.QueryRow("SELECT id, nome, COALESCE(indirizzo, ''), COALESCE(citta, ''), COALESCE(cap, ''), COALESCE(telefono, ''), COALESCE(email, ''), COALESCE(note, '') FROM uffici WHERE id = ?", id).
			Scan(&u.ID, &u.Nome, &u.Indirizzo, &u.Citta, &u.CAP, &u.Telefono, &u.Email, &u.Note)
		if err != nil {
			http.Redirect(w, r, "/uffici", http.StatusSeeOther)
			return
		}
		data := NewPageData("Modifica Ufficio - FurvioGest", r)
		data.Data = u
		renderTemplate(w, "ufficio_form.html", data)
		return
	}

	r.ParseForm()
	nome := strings.TrimSpace(r.FormValue("nome"))
	indirizzo := strings.TrimSpace(r.FormValue("indirizzo"))
	citta := strings.TrimSpace(r.FormValue("citta"))
	cap := strings.TrimSpace(r.FormValue("cap"))
	telefono := strings.TrimSpace(r.FormValue("telefono"))
	email := strings.TrimSpace(r.FormValue("email"))
	note := strings.TrimSpace(r.FormValue("note"))

	database.DB.Exec("UPDATE uffici SET nome = ?, indirizzo = ?, citta = ?, cap = ?, telefono = ?, email = ?, note = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		nome, indirizzo, citta, cap, telefono, email, note, id)

	http.Redirect(w, r, "/uffici", http.StatusSeeOther)
}

func EliminaUfficio(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/uffici/elimina/")
	id, _ := strconv.ParseInt(path, 10, 64)

	database.DB.Exec("DELETE FROM uffici WHERE id = ?", id)
	http.Redirect(w, r, "/uffici", http.StatusSeeOther)
}

// ============================================
// GESTIONE RETE UFFICIO
// ============================================

func GestioneReteUfficio(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/uffici/rete/")
	ufficioID, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/uffici", http.StatusSeeOther)
		return
	}

	// Carica ufficio
	var ufficio Ufficio
	err = database.DB.QueryRow("SELECT id, nome, COALESCE(indirizzo, ''), COALESCE(citta, '') FROM uffici WHERE id = ?", ufficioID).
		Scan(&ufficio.ID, &ufficio.Nome, &ufficio.Indirizzo, &ufficio.Citta)
	if err != nil {
		http.Redirect(w, r, "/uffici", http.StatusSeeOther)
		return
	}

	// Carica AC (può essere nil)
	ac := getACUfficioByID(ufficioID)

	// Carica switch
	switches := getSwitchesUfficio(ufficioID)

	// Carica backup recenti
	backups := getBackupsUfficio(ufficioID, 10)

	pageData := UfficioPageData{
		Ufficio:  ufficio,
		AC:       ac,
		Switches: switches,
		Backups:  backups,
	}

	data := NewPageData("Gestione Rete - "+ufficio.Nome, r)
	data.Data = pageData
	renderTemplate(w, "rete_ufficio.html", data)
}

// ============================================
// AC UFFICIO
// ============================================

func SalvaACUfficio(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metodo non consentito", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/uffici/ac/salva/")
	ufficioID, _ := strconv.ParseInt(path, 10, 64)

	r.ParseForm()
	ip := strings.TrimSpace(r.FormValue("ip"))
	sshPort, _ := strconv.Atoi(r.FormValue("ssh_port"))
	if sshPort == 0 {
		sshPort = 22
	}
	sshUser := strings.TrimSpace(r.FormValue("ssh_user"))
	sshPass := strings.TrimSpace(r.FormValue("ssh_pass"))
	protocollo := r.FormValue("protocollo")
	if protocollo == "" {
		protocollo = "ssh"
	}
	note := strings.TrimSpace(r.FormValue("note"))

	var existingID int64
	err := database.DB.QueryRow("SELECT id FROM ac_ufficio WHERE ufficio_id = ?", ufficioID).Scan(&existingID)

	if err == sql.ErrNoRows {
		_, err = database.DB.Exec("INSERT INTO ac_ufficio (ufficio_id, ip, ssh_port, ssh_user, ssh_pass, protocollo, note) VALUES (?, ?, ?, ?, ?, ?, ?)",
			ufficioID, ip, sshPort, sshUser, sshPass, protocollo, note)
	} else if err == nil {
		_, err = database.DB.Exec("UPDATE ac_ufficio SET ip = ?, ssh_port = ?, ssh_user = ?, ssh_pass = ?, protocollo = ?, note = ?, updated_at = CURRENT_TIMESTAMP WHERE ufficio_id = ?",
			ip, sshPort, sshUser, sshPass, protocollo, note, ufficioID)
	}

	http.Redirect(w, r, fmt.Sprintf("/uffici/rete/%d", ufficioID), http.StatusSeeOther)
}

func EliminaACUfficio(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/uffici/ac/elimina/")
	ufficioID, _ := strconv.ParseInt(path, 10, 64)

	database.DB.Exec("DELETE FROM ac_ufficio WHERE ufficio_id = ?", ufficioID)
	http.Redirect(w, r, fmt.Sprintf("/uffici/rete/%d", ufficioID), http.StatusSeeOther)
}

// ============================================
// SWITCH UFFICIO
// ============================================

func NuovoSwitchUfficio(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metodo non consentito", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/uffici/switch/nuovo/")
	ufficioID, _ := strconv.ParseInt(path, 10, 64)

	r.ParseForm()
	marca := r.FormValue("marca")
	modello := strings.TrimSpace(r.FormValue("modello"))
	ip := strings.TrimSpace(r.FormValue("ip"))
	sshPort, _ := strconv.Atoi(r.FormValue("ssh_port"))
	if sshPort == 0 {
		sshPort = 22
	}
	sshUser := strings.TrimSpace(r.FormValue("ssh_user"))
	sshPass := strings.TrimSpace(r.FormValue("ssh_pass"))
	note := strings.TrimSpace(r.FormValue("note"))
	protocollo := r.FormValue("protocollo")
	if protocollo == "" {
		protocollo = "ssh"
	}

	// Recupera hostname automaticamente
	nome := getSwitchHostname(ip, sshPort, sshUser, sshPass, marca, protocollo)
	log.Printf("[NUOVO SWITCH UFFICIO] Hostname recuperato: %s", nome)

	_, err := database.DB.Exec("INSERT INTO switch_ufficio (ufficio_id, nome, marca, modello, ip, ssh_port, ssh_user, ssh_pass, note, protocollo) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		ufficioID, nome, marca, modello, ip, sshPort, sshUser, sshPass, note, protocollo)

	if err != nil {
		http.Error(w, "Errore salvataggio: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/uffici/rete/%d", ufficioID), http.StatusSeeOther)
}

func ModificaSwitchUfficio(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metodo non consentito", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/uffici/switch/modifica/")
	switchID, _ := strconv.ParseInt(path, 10, 64)

	var ufficioID int64
	database.DB.QueryRow("SELECT ufficio_id FROM switch_ufficio WHERE id = ?", switchID).Scan(&ufficioID)

	r.ParseForm()
	marca := r.FormValue("marca")
	modello := strings.TrimSpace(r.FormValue("modello"))
	ip := strings.TrimSpace(r.FormValue("ip"))
	sshPort, _ := strconv.Atoi(r.FormValue("ssh_port"))
	if sshPort == 0 {
		sshPort = 22
	}
	sshUser := strings.TrimSpace(r.FormValue("ssh_user"))
	sshPass := strings.TrimSpace(r.FormValue("ssh_pass"))
	note := strings.TrimSpace(r.FormValue("note"))
	protocollo := r.FormValue("protocollo")
	if protocollo == "" {
		protocollo = "ssh"
	}

	database.DB.Exec("UPDATE switch_ufficio SET marca = ?, modello = ?, ip = ?, ssh_port = ?, ssh_user = ?, ssh_pass = ?, note = ?, protocollo = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		marca, modello, ip, sshPort, sshUser, sshPass, note, protocollo, switchID)

	http.Redirect(w, r, fmt.Sprintf("/uffici/rete/%d", ufficioID), http.StatusSeeOther)
}

func EliminaSwitchUfficio(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/uffici/switch/elimina/")
	switchID, _ := strconv.ParseInt(path, 10, 64)

	var ufficioID int64
	database.DB.QueryRow("SELECT ufficio_id FROM switch_ufficio WHERE id = ?", switchID).Scan(&ufficioID)

	database.DB.Exec("DELETE FROM switch_ufficio WHERE id = ?", switchID)
	http.Redirect(w, r, fmt.Sprintf("/uffici/rete/%d", ufficioID), http.StatusSeeOther)
}

// ============================================
// FUNZIONI HELPER
// ============================================

func getACUfficioByID(ufficioID int64) *ACUfficio {
	var ac ACUfficio
	var ultimoBackup sql.NullString
	err := database.DB.QueryRow("SELECT id, ufficio_id, ip, ssh_port, ssh_user, ssh_pass, COALESCE(protocollo, 'ssh'), COALESCE(modello, ''), COALESCE(note, ''), ultimo_backup FROM ac_ufficio WHERE ufficio_id = ?", ufficioID).
		Scan(&ac.ID, &ac.UfficioID, &ac.IP, &ac.SSHPort, &ac.SSHUser, &ac.SSHPass, &ac.Protocollo, &ac.Modello, &ac.Note, &ultimoBackup)
	if err != nil {
		return nil
	}
	if ultimoBackup.Valid {
		ac.UltimoBackup = ultimoBackup.String
	}
	return &ac
}

func getSwitchesUfficio(ufficioID int64) []SwitchUfficio {
	var switches []SwitchUfficio
	rows, err := database.DB.Query("SELECT id, ufficio_id, nome, marca, COALESCE(modello, ''), ip, ssh_port, ssh_user, ssh_pass, COALESCE(protocollo, 'ssh'), COALESCE(note, ''), ultimo_backup FROM switch_ufficio WHERE ufficio_id = ? ORDER BY nome", ufficioID)
	if err != nil {
		return switches
	}
	defer rows.Close()

	for rows.Next() {
		var sw SwitchUfficio
		var ultimoBackup sql.NullString
		rows.Scan(&sw.ID, &sw.UfficioID, &sw.Nome, &sw.Marca, &sw.Modello, &sw.IP, &sw.SSHPort, &sw.SSHUser, &sw.SSHPass, &sw.Protocollo, &sw.Note, &ultimoBackup)
		if ultimoBackup.Valid {
			sw.UltimoBackup = ultimoBackup.String
		}
		switches = append(switches, sw)
	}
	return switches
}

func getBackupsUfficio(ufficioID int64, limit int) []ConfigBackup {
	var backups []ConfigBackup
	rows, err := database.DB.Query("SELECT id, COALESCE(ufficio_id, 0), tipo_apparato, apparato_id, nome_apparato, file_path, file_size, hash_md5, created_at FROM config_backup_ufficio WHERE ufficio_id = ? ORDER BY created_at DESC LIMIT ?", ufficioID, limit)
	if err != nil {
		return backups
	}
	defer rows.Close()

	for rows.Next() {
		var b ConfigBackup
		rows.Scan(&b.ID, &b.NaveID, &b.TipoApparato, &b.ApparatoID, &b.NomeApparato, &b.FilePath, &b.FileSize, &b.HashMD5, &b.CreatedAt)
		backups = append(backups, b)
	}
	return backups
}

// Funzione per evitare errore declared and not used
var _ = template.HTML("")

// ============================================
// API BACKUP UFFICI E SALE SERVER
// ============================================

// APIBackupConfigUfficio esegue backup configurazione per uffici/sale server
func APIBackupConfigUfficio(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tipoApparato := r.URL.Query().Get("tipo") // ac_ufficio, switch_ufficio, switch_sala_server
	apparatoIDStr := r.URL.Query().Get("id")
	apparatoID, _ := strconv.ParseInt(apparatoIDStr, 10, 64)

	result := map[string]interface{}{
		"success": false,
		"message": "",
	}

	var ip, sshUser, sshPass, nome, protocollo string
	var sshPort int
	var ufficioID, salaServerID int64
	var cmdStr string

	switch tipoApparato {
	case "ac_ufficio":
		var ac ACUfficio
		err := database.DB.QueryRow("SELECT id, ufficio_id, ip, ssh_port, ssh_user, ssh_pass, COALESCE(protocollo, 'ssh') FROM ac_ufficio WHERE id = ?", apparatoID).
			Scan(&ac.ID, &ac.UfficioID, &ac.IP, &ac.SSHPort, &ac.SSHUser, &ac.SSHPass, &ac.Protocollo)
		if err != nil {
			result["message"] = "AC non trovato"
			json.NewEncoder(w).Encode(result)
			return
		}
		ip = ac.IP
		sshUser = ac.SSHUser
		sshPass = ac.SSHPass
		sshPort = ac.SSHPort
		protocollo = ac.Protocollo
		ufficioID = ac.UfficioID
		nome = "AC"
		cmdStr = "display current-configuration"

	case "switch_ufficio":
		var sw SwitchUfficio
		err := database.DB.QueryRow("SELECT id, ufficio_id, nome, ip, ssh_port, ssh_user, ssh_pass, COALESCE(protocollo, 'ssh'), marca FROM switch_ufficio WHERE id = ?", apparatoID).
			Scan(&sw.ID, &sw.UfficioID, &sw.Nome, &sw.IP, &sw.SSHPort, &sw.SSHUser, &sw.SSHPass, &sw.Protocollo, &sw.Marca)
		if err != nil {
			result["message"] = "Switch non trovato"
			json.NewEncoder(w).Encode(result)
			return
		}
		ip = sw.IP
		sshUser = sw.SSHUser
		sshPass = sw.SSHPass
		sshPort = sw.SSHPort
		protocollo = sw.Protocollo
		ufficioID = sw.UfficioID
		nome = sw.Nome
		if sw.Marca == "huawei" {
			cmdStr = "display current-configuration"
		} else {
			cmdStr = "show running-config"
		}

	case "switch_sala_server":
		var sw SwitchSalaServer
		err := database.DB.QueryRow("SELECT id, sala_server_id, nome, ip, ssh_port, ssh_user, ssh_pass, COALESCE(protocollo, 'ssh'), marca FROM switch_sala_server WHERE id = ?", apparatoID).
			Scan(&sw.ID, &sw.SalaServerID, &sw.Nome, &sw.IP, &sw.SSHPort, &sw.SSHUser, &sw.SSHPass, &sw.Protocollo, &sw.Marca)
		if err != nil {
			result["message"] = "Switch non trovato"
			json.NewEncoder(w).Encode(result)
			return
		}
		ip = sw.IP
		sshUser = sw.SSHUser
		sshPass = sw.SSHPass
		sshPort = sw.SSHPort
		protocollo = sw.Protocollo
		salaServerID = sw.SalaServerID
		nome = sw.Nome
		if sw.Marca == "huawei" {
			cmdStr = "display current-configuration"
		} else {
			cmdStr = "show running-config"
		}

	default:
		result["message"] = "Tipo apparato non valido"
		json.NewEncoder(w).Encode(result)
		return
	}

	// Esegui backup
	var output string
	var err error
	if protocollo == "telnet" {
		output, err = executeTelnetCommand(ip, sshUser, sshPass, cmdStr)
	} else {
		output, err = executeSSHBackup(ip, sshPort, sshUser, sshPass, cmdStr)
	}

	if err != nil {
		result["message"] = "Errore backup: " + err.Error()
		json.NewEncoder(w).Encode(result)
		return
	}

	// Calcola hash MD5
	hash := md5.Sum([]byte(output))
	hashStr := hex.EncodeToString(hash[:])

	// Salva file
	var backupDir string
	if ufficioID > 0 {
		backupDir = filepath.Join("data", "backups", fmt.Sprintf("ufficio_%d", ufficioID))
	} else {
		backupDir = filepath.Join("data", "backups", fmt.Sprintf("sala_server_%d", salaServerID))
	}
	os.MkdirAll(backupDir, 0755)

	timestamp := time.Now().Format("20060102_150405")
	tipoShort := strings.Replace(tipoApparato, "_ufficio", "", 1)
	tipoShort = strings.Replace(tipoShort, "_sala_server", "", 1)
	filename := fmt.Sprintf("%s_%s_%s.cfg", tipoShort, nome, timestamp)
	filePath := filepath.Join(backupDir, filename)

	err = os.WriteFile(filePath, []byte(output), 0644)
	if err != nil {
		result["message"] = "Errore salvataggio file: " + err.Error()
		json.NewEncoder(w).Encode(result)
		return
	}

	// Inserisci record DB
	var ufficioIDPtr, salaServerIDPtr interface{}
	if ufficioID > 0 {
		ufficioIDPtr = ufficioID
	}
	if salaServerID > 0 {
		salaServerIDPtr = salaServerID
	}

	_, err = database.DB.Exec("INSERT INTO config_backup_ufficio (ufficio_id, sala_server_id, tipo_apparato, apparato_id, nome_apparato, file_path, file_size, hash_md5) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		ufficioIDPtr, salaServerIDPtr, tipoShort, apparatoID, nome, filePath, len(output), hashStr)

	if err != nil {
		result["message"] = "Errore DB: " + err.Error()
		json.NewEncoder(w).Encode(result)
		return
	}

	// Aggiorna timestamp ultimo backup
	switch tipoApparato {
	case "ac_ufficio":
		database.DB.Exec("UPDATE ac_ufficio SET ultimo_backup = CURRENT_TIMESTAMP WHERE id = ?", apparatoID)
	case "switch_ufficio":
		database.DB.Exec("UPDATE switch_ufficio SET ultimo_backup = CURRENT_TIMESTAMP WHERE id = ?", apparatoID)
	case "switch_sala_server":
		database.DB.Exec("UPDATE switch_sala_server SET ultimo_backup = CURRENT_TIMESTAMP WHERE id = ?", apparatoID)
	}

	result["success"] = true
	result["message"] = "Backup completato"
	json.NewEncoder(w).Encode(result)
}

// APIDownloadBackupUfficio serve download backup ufficio/sala server
func APIDownloadBackupUfficio(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/rete/download-backup-ufficio/")
	backupID, _ := strconv.ParseInt(path, 10, 64)

	var filePath, nomeApparato string
	err := database.DB.QueryRow("SELECT file_path, nome_apparato FROM config_backup_ufficio WHERE id = ?", backupID).
		Scan(&filePath, &nomeApparato)

	if err != nil {
		http.Error(w, "Backup non trovato", http.StatusNotFound)
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "File non trovato", http.StatusNotFound)
		return
	}
	defer file.Close()

	filename := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath)) + ".txt"
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "application/octet-stream")

	io.Copy(w, file)
}
