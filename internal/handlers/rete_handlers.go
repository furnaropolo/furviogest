package handlers

import (
	"bytes"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"furviogest/internal/database"
)

// ============================================
// STRUTTURE DATI
// ============================================

type AccessController struct {
	ID           int64
	NaveID       int64
	IP           string
	SSHPort      int
	SSHUser      string
	SSHPass      string
	Note         string
	UltimoCheck  string
	UltimoBackup string
}

type SwitchNave struct {
	ID           int64
	NaveID       int64
	Nome         string
	Marca        string // huawei o hp
	Modello      string
	IP           string
	SSHPort      int
	SSHUser      string
	SSHPass      string
	Note         string
	UltimoCheck  string
	UltimoBackup string
}

type AccessPoint struct {
	ID          int64
	NaveID      int64
	ACID        int64
	APName      string
	APMAC       string
	APModel     string
	APSerial    string
	APIP        string
	SwitchID    *int64
	SwitchPort  string
	Stato       string // online, offline, fault, unknown
	UltimoCheck string
	// Campi join
	SwitchNome  string
}

type ConfigBackup struct {
	ID            int64
	NaveID        int64
	TipoApparato  string // ac o switch
	ApparatoID    int64
	NomeApparato  string
	FilePath      string
	FileSize      int64
	HashMD5       string
	CreatedAt     string
}

type ReteNavePageData struct {
	Nave         NaveInfo
	AC           *AccessController
	Switches     []SwitchNave
	AccessPoints []AccessPoint
	Backups      []ConfigBackup
	APFault      int // Conteggio AP in fault
}

type NaveInfo struct {
	ID             int64
	Nome           string
	NomeCompagnia  string
	FermaPerLavori bool
}

// ============================================
// HANDLERS PAGINA GESTIONE RETE
// ============================================

// GestioneReteNave mostra la pagina gestione rete di una nave

// executeSSHCommand esegue un comando SSH usando expect per compatibilità Huawei
func executeSSHCommand(ip string, port int, user, pass, command string) (string, error) {
	expectScript := fmt.Sprintf(`
log_user 1
set timeout 60
spawn ssh -o StrictHostKeyChecking=no -o ConnectTimeout=30 -p %d %s@%s

expect {
    "assword:" {
        send "%s\r"
        exp_continue
    }
    ">" {
        send "%s\r"
    }
    "<*>" {
        send "%s\r"
    }
    timeout {
        exit 4
    }
    "Permission denied" {
        exit 1
    }
}

expect {
    ">" {
        send "quit\r"
    }
    "<*>" {
        send "quit\r"
    }
    timeout {}
}

expect eof
`, port, user, ip, pass, command, command)

	cmd := exec.Command("expect", "-c", expectScript)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("SSH error: %v - %s", err, stderr.String())
	}

	return stdout.String(), nil
}

func GestioneReteNave_OLD(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Gestione Rete Nave - FurvioGest", r)

	// Estrai ID nave dall'URL
	path := strings.TrimPrefix(r.URL.Path, "/navi/rete/")
	naveID, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	// Carica dati nave
	nave := getNaveInfoByID(naveID)
	if nave.ID == 0 {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	// Carica AC (può essere nil)
	ac := getAccessControllerByNave(naveID)

	// Carica switch
	switches := getSwitchesByNave(naveID)

	// Carica AP
	accessPoints := getAccessPointsByNave(naveID)

	// Carica backup recenti
	backups := getRecentBackups(naveID, 10)

	// Conta AP in fault
	apFault := countAPByStatus(naveID, "fault")

	pageData := ReteNavePageData{
		Nave:         nave,
		AC:           ac,
		Switches:     switches,
		AccessPoints: accessPoints,
		Backups:      backups,
		APFault:      apFault,
	}

	data.Data = pageData
	renderTemplate(w, "rete_nave.html", data)
}

// SalvaAccessController salva o aggiorna l'AC della nave
func SalvaAccessController(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metodo non consentito", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/navi/ac/salva/")
	naveID, _ := strconv.ParseInt(path, 10, 64)

	r.ParseForm()
	ip := strings.TrimSpace(r.FormValue("ip"))
	sshPort, _ := strconv.Atoi(r.FormValue("ssh_port"))
	if sshPort == 0 {
		sshPort = 22
	}
	sshUser := strings.TrimSpace(r.FormValue("ssh_user"))
	sshPass := strings.TrimSpace(r.FormValue("ssh_pass"))
	note := strings.TrimSpace(r.FormValue("note"))

	// Verifica se esiste già un AC per questa nave
	var existingID int64
	err := database.DB.QueryRow("SELECT id FROM access_controller WHERE nave_id = ?", naveID).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Inserisci nuovo
		_, err = database.DB.Exec(`
			INSERT INTO access_controller (nave_id, ip, ssh_port, ssh_user, ssh_pass, note)
			VALUES (?, ?, ?, ?, ?, ?)
		`, naveID, ip, sshPort, sshUser, sshPass, note)
	} else if err == nil {
		// Aggiorna esistente
		_, err = database.DB.Exec(`
			UPDATE access_controller SET ip = ?, ssh_port = ?, ssh_user = ?, ssh_pass = ?, note = ?, updated_at = CURRENT_TIMESTAMP
			WHERE nave_id = ?
		`, ip, sshPort, sshUser, sshPass, note, naveID)
	}

	if err != nil {
		http.Error(w, "Errore salvataggio: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/navi/rete/%d", naveID), http.StatusSeeOther)
}

// EliminaAccessController elimina l'AC della nave
func EliminaAccessController(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/navi/ac/elimina/")
	naveID, _ := strconv.ParseInt(path, 10, 64)

	database.DB.Exec("DELETE FROM access_controller WHERE nave_id = ?", naveID)
	// Elimina anche gli AP associati
	database.DB.Exec("DELETE FROM access_point WHERE nave_id = ?", naveID)

	http.Redirect(w, r, fmt.Sprintf("/navi/rete/%d", naveID), http.StatusSeeOther)
}

// NuovoSwitch gestisce la creazione di un nuovo switch
func NuovoSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metodo non consentito", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/navi/switch/nuovo/")
	naveID, _ := strconv.ParseInt(path, 10, 64)

	r.ParseForm()
	nome := strings.TrimSpace(r.FormValue("nome"))
	marca := r.FormValue("marca") // huawei o hp
	modello := strings.TrimSpace(r.FormValue("modello"))
	ip := strings.TrimSpace(r.FormValue("ip"))
	sshPort, _ := strconv.Atoi(r.FormValue("ssh_port"))
	if sshPort == 0 {
		sshPort = 22
	}
	sshUser := strings.TrimSpace(r.FormValue("ssh_user"))
	sshPass := strings.TrimSpace(r.FormValue("ssh_pass"))
	note := strings.TrimSpace(r.FormValue("note"))

	_, err := database.DB.Exec(`
		INSERT INTO switch_nave (nave_id, nome, marca, modello, ip, ssh_port, ssh_user, ssh_pass, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, naveID, nome, marca, modello, ip, sshPort, sshUser, sshPass, note)

	if err != nil {
		http.Error(w, "Errore salvataggio: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/navi/rete/%d", naveID), http.StatusSeeOther)
}

// ModificaSwitch gestisce la modifica di uno switch
func ModificaSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metodo non consentito", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/navi/switch/modifica/")
	switchID, _ := strconv.ParseInt(path, 10, 64)

	r.ParseForm()
	naveID, _ := strconv.ParseInt(r.FormValue("nave_id"), 10, 64)
	nome := strings.TrimSpace(r.FormValue("nome"))
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

	_, err := database.DB.Exec(`
		UPDATE switch_nave SET nome = ?, marca = ?, modello = ?, ip = ?, ssh_port = ?, ssh_user = ?, ssh_pass = ?, note = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, nome, marca, modello, ip, sshPort, sshUser, sshPass, note, switchID)

	if err != nil {
		http.Error(w, "Errore salvataggio: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/navi/rete/%d", naveID), http.StatusSeeOther)
}

// EliminaSwitch elimina uno switch
func EliminaSwitch(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/navi/switch/elimina/")
	switchID, _ := strconv.ParseInt(path, 10, 64)

	// Ottieni naveID prima di eliminare
	var naveID int64
	database.DB.QueryRow("SELECT nave_id FROM switch_nave WHERE id = ?", switchID).Scan(&naveID)

	// Elimina backup e file associati
	rows, _ := database.DB.Query("SELECT file_path FROM config_backup WHERE tipo_apparato = 'switch' AND apparato_id = ?", switchID)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var filePath string
			rows.Scan(&filePath)
			os.Remove(filePath)
		}
	}
	database.DB.Exec("DELETE FROM config_backup WHERE tipo_apparato = 'switch' AND apparato_id = ?", switchID)
	database.DB.Exec("DELETE FROM switch_nave WHERE id = ?", switchID)

	http.Redirect(w, r, fmt.Sprintf("/navi/rete/%d", naveID), http.StatusSeeOther)
}

// ============================================
// API MONITORAGGIO
// ============================================

// APIScanAccessPoints esegue scan AP dall'AC Huawei
func APIScanAccessPoints(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	naveIDStr := r.URL.Query().Get("nave_id")
	naveID, _ := strconv.ParseInt(naveIDStr, 10, 64)

	result := map[string]interface{}{
		"success": false,
		"message": "",
		"aps":     []map[string]string{},
	}

	// Verifica che la nave non sia ferma per lavori
	nave := getNaveInfoByID(naveID)
	if nave.FermaPerLavori {
		result["message"] = "Nave ferma per lavori - monitoraggio disabilitato"
		json.NewEncoder(w).Encode(result)
		return
	}

	// Ottieni AC
	ac := getAccessControllerByNave(naveID)
	if ac == nil {
		result["message"] = "Access Controller non configurato"
		json.NewEncoder(w).Encode(result)
		return
	}

	// Esegui comando SSH su AC Huawei usando expect
	output, err := executeSSHCommand(ac.IP, ac.SSHPort, ac.SSHUser, ac.SSHPass, "display ap all")
	if err != nil {
		result["message"] = "Errore connessione SSH: " + err.Error()
		json.NewEncoder(w).Encode(result)
		return
	}

	// Parse output Huawei AC
	aps := parseHuaweiAPOutput(string(output))

	// Aggiorna database
	for _, ap := range aps {
		updateOrCreateAP(naveID, ac.ID, ap)
	}

	// Aggiorna timestamp ultimo check
	database.DB.Exec("UPDATE access_controller SET ultimo_check = CURRENT_TIMESTAMP WHERE id = ?", ac.ID)

	result["success"] = true
	result["message"] = fmt.Sprintf("Trovati %d Access Point", len(aps))
	result["aps"] = aps

	json.NewEncoder(w).Encode(result)
}

// APIScanMacTable esegue scan tabella MAC da uno switch
func APIScanMacTable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switchIDStr := r.URL.Query().Get("switch_id")
	switchID, _ := strconv.ParseInt(switchIDStr, 10, 64)

	result := map[string]interface{}{
		"success": false,
		"message": "",
		"entries": []map[string]string{},
	}

	sw := getSwitchByID(switchID)
	if sw == nil {
		result["message"] = "Switch non trovato"
		json.NewEncoder(w).Encode(result)
		return
	}

	// Comando diverso in base alla marca
	var cmdStr string
	if sw.Marca == "huawei" {
		cmdStr = "display mac-address"
	} else { // hp
		cmdStr = "show mac-address"
	}

	// Esegui comando SSH usando expect
	output, err := executeSSHCommand(sw.IP, sw.SSHPort, sw.SSHUser, sw.SSHPass, cmdStr)
	if err != nil {
		result["message"] = "Errore connessione SSH: " + err.Error()
		json.NewEncoder(w).Encode(result)
		return
	}

	// Parse output
	var entries []map[string]string
	if sw.Marca == "huawei" {
		entries = parseHuaweiMacTable(string(output))
	} else {
		entries = parseHPMacTable(string(output))
	}

	// Aggiorna porta degli AP in base al MAC
	for _, entry := range entries {
		mac := entry["mac"]
		port := entry["port"]
		updateAPSwitchPort(sw.NaveID, switchID, mac, port)
	}

	// Aggiorna timestamp
	database.DB.Exec("UPDATE switch_nave SET ultimo_check = CURRENT_TIMESTAMP WHERE id = ?", switchID)

	result["success"] = true
	result["message"] = fmt.Sprintf("Trovate %d entry MAC", len(entries))
	result["entries"] = entries

	json.NewEncoder(w).Encode(result)
}

// APIBackupConfig esegue backup configurazione
func APIBackupConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tipoApparato := r.URL.Query().Get("tipo") // ac o switch
	apparatoIDStr := r.URL.Query().Get("id")
	apparatoID, _ := strconv.ParseInt(apparatoIDStr, 10, 64)

	result := map[string]interface{}{
		"success": false,
		"message": "",
	}

	var ip, sshUser, sshPass, nome string
	var sshPort int
	var naveID int64
	var cmdStr string

	if tipoApparato == "ac" {
		var ac AccessController
		err := database.DB.QueryRow(`
			SELECT id, nave_id, ip, ssh_port, ssh_user, ssh_pass FROM access_controller WHERE id = ?
		`, apparatoID).Scan(&ac.ID, &ac.NaveID, &ac.IP, &ac.SSHPort, &ac.SSHUser, &ac.SSHPass)
		if err != nil {
			result["message"] = "AC non trovato"
			json.NewEncoder(w).Encode(result)
			return
		}
		ip = ac.IP
		sshUser = ac.SSHUser
		sshPass = ac.SSHPass
		sshPort = ac.SSHPort
		naveID = ac.NaveID
		nome = "AC"
		cmdStr = "display current-configuration"
	} else {
		sw := getSwitchByID(apparatoID)
		if sw == nil {
			result["message"] = "Switch non trovato"
			json.NewEncoder(w).Encode(result)
			return
		}
		ip = sw.IP
		sshUser = sw.SSHUser
		sshPass = sw.SSHPass
		sshPort = sw.SSHPort
		naveID = sw.NaveID
		nome = sw.Nome
		if sw.Marca == "huawei" {
			cmdStr = "display current-configuration"
		} else {
			cmdStr = "show running-config"
		}
	}

	// Esegui comando backup usando expect
	output, err := executeSSHCommand(ip, sshPort, sshUser, sshPass, cmdStr)
	if err != nil {
		result["message"] = "Errore backup: " + err.Error()
		json.NewEncoder(w).Encode(result)
		return
	}

	// Calcola hash MD5
	hash := md5.Sum([]byte(output))
	hashStr := hex.EncodeToString(hash[:])

	// Verifica se config è cambiata rispetto all'ultimo backup
	var lastHash string
	database.DB.QueryRow(`
		SELECT hash_md5 FROM config_backup
		WHERE nave_id = ? AND tipo_apparato = ? AND apparato_id = ?
		ORDER BY created_at DESC LIMIT 1
	`, naveID, tipoApparato, apparatoID).Scan(&lastHash)

	if lastHash == hashStr {
		result["success"] = true
		result["message"] = "Configurazione non cambiata dall'ultimo backup"
		json.NewEncoder(w).Encode(result)
		return
	}

	// Salva file
	backupDir := filepath.Join("data", "backups", fmt.Sprintf("nave_%d", naveID))
	os.MkdirAll(backupDir, 0755)

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_%s.cfg", tipoApparato, nome, timestamp)
	filePath := filepath.Join(backupDir, filename)

	err = os.WriteFile(filePath, []byte(output), 0644)
	if err != nil {
		result["message"] = "Errore salvataggio file: " + err.Error()
		json.NewEncoder(w).Encode(result)
		return
	}

	// Inserisci record DB
	_, err = database.DB.Exec(`
		INSERT INTO config_backup (nave_id, tipo_apparato, apparato_id, nome_apparato, file_path, file_size, hash_md5)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, naveID, tipoApparato, apparatoID, nome, filePath, len(output), hashStr)

	if err != nil {
		result["message"] = "Errore DB: " + err.Error()
		json.NewEncoder(w).Encode(result)
		return
	}

	// Aggiorna timestamp ultimo backup
	if tipoApparato == "ac" {
		database.DB.Exec("UPDATE access_controller SET ultimo_backup = CURRENT_TIMESTAMP WHERE id = ?", apparatoID)
	} else {
		database.DB.Exec("UPDATE switch_nave SET ultimo_backup = CURRENT_TIMESTAMP WHERE id = ?", apparatoID)
	}

	result["success"] = true
	result["message"] = "Backup completato"
	json.NewEncoder(w).Encode(result)
}

// APIDownloadConfig serve il download di un backup
func APIDownloadConfig(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/rete/download-config/")
	backupID, _ := strconv.ParseInt(path, 10, 64)

	var filePath, nomeApparato string
	err := database.DB.QueryRow(`
		SELECT file_path, nome_apparato FROM config_backup WHERE id = ?
	`, backupID).Scan(&filePath, &nomeApparato)

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

	// Imposta header per download
	// Forza estensione .txt per leggibilità
	filename := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath)) + ".txt"
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "application/octet-stream")

	io.Copy(w, file)
}

// APITestSSH testa la connessione SSH
func APITestSSH(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ip := r.URL.Query().Get("ip")
	port := r.URL.Query().Get("port")
	user := r.URL.Query().Get("user")
	pass := r.URL.Query().Get("pass")

	if port == "" {
		port = "22"
	}

	result := map[string]interface{}{
		"success": false,
		"message": "",
	}

	// Usa expect per compatibilità con Huawei
	expectScript := fmt.Sprintf(`
set timeout 15
spawn ssh -o StrictHostKeyChecking=no -p %s %s@%s
expect {
    "*assword*" { send "%s\r"; exp_continue }
    "<*>" { exit 0 }
    ">" { exit 0 }
    "Permission denied" { exit 1 }
    "Connection refused" { exit 2 }
    "Connection timed out" { exit 3 }
    timeout { exit 4 }
    eof { exit 0 }
}
`, port, user, ip, pass)

	cmd := exec.Command("expect", "-c", expectScript)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	
	log.Printf("[SSH TEST] IP: %s, User: %s, ExitCode: %v", ip, user, cmd.ProcessState.ExitCode())

	exitCode := cmd.ProcessState.ExitCode()
	
	if exitCode == 0 && (strings.Contains(outputStr, "<") || strings.Contains(outputStr, ">") || strings.Contains(outputStr, "Huawei") || strings.Contains(outputStr, "HUAWEI")) {
		result["success"] = true
		result["message"] = "Connessione SSH riuscita"
	} else if exitCode == 1 || strings.Contains(outputStr, "Permission denied") {
		result["message"] = "Password errata o utente non valido"
	} else if exitCode == 2 || strings.Contains(outputStr, "Connection refused") {
		result["message"] = "Connessione rifiutata - porta SSH chiusa"
	} else if exitCode == 3 || exitCode == 4 || strings.Contains(outputStr, "timed out") {
		result["message"] = "Timeout - apparato non raggiungibile"
	} else if err != nil {
		result["message"] = "Errore: " + err.Error()
	} else {
		result["message"] = "Risposta inattesa"
	}

	json.NewEncoder(w).Encode(result)
}

// ============================================
// FUNZIONI HELPER
// ============================================

func getNaveInfoByID(id int64) NaveInfo {
	var nave NaveInfo
	var ferma int
	database.DB.QueryRow(`
		SELECT n.id, n.nome, COALESCE(c.nome, '') as compagnia, COALESCE(n.ferma_per_lavori, 0)
		FROM navi n
		LEFT JOIN compagnie c ON n.compagnia_id = c.id
		WHERE n.id = ?
	`, id).Scan(&nave.ID, &nave.Nome, &nave.NomeCompagnia, &ferma)
	nave.FermaPerLavori = ferma == 1
	return nave
}

func getAccessControllerByNave(naveID int64) *AccessController {
	var ac AccessController
	var ultimoCheck, ultimoBackup sql.NullString
	err := database.DB.QueryRow(`
		SELECT id, nave_id, ip, ssh_port, ssh_user, ssh_pass, COALESCE(note, ''), ultimo_check, ultimo_backup
		FROM access_controller WHERE nave_id = ?
	`, naveID).Scan(&ac.ID, &ac.NaveID, &ac.IP, &ac.SSHPort, &ac.SSHUser, &ac.SSHPass, &ac.Note, &ultimoCheck, &ultimoBackup)
	if err != nil {
		return nil
	}
	if ultimoCheck.Valid {
		ac.UltimoCheck = ultimoCheck.String
	}
	if ultimoBackup.Valid {
		ac.UltimoBackup = ultimoBackup.String
	}
	return &ac
}

func getSwitchesByNave(naveID int64) []SwitchNave {
	var switches []SwitchNave
	rows, err := database.DB.Query(`
		SELECT id, nave_id, nome, marca, COALESCE(modello, ''), ip, ssh_port, ssh_user, ssh_pass, COALESCE(note, ''), ultimo_check, ultimo_backup
		FROM switch_nave WHERE nave_id = ? ORDER BY nome
	`, naveID)
	if err != nil {
		return switches
	}
	defer rows.Close()

	for rows.Next() {
		var sw SwitchNave
		var ultimoCheck, ultimoBackup sql.NullString
		rows.Scan(&sw.ID, &sw.NaveID, &sw.Nome, &sw.Marca, &sw.Modello, &sw.IP, &sw.SSHPort, &sw.SSHUser, &sw.SSHPass, &sw.Note, &ultimoCheck, &ultimoBackup)
		if ultimoCheck.Valid {
			sw.UltimoCheck = ultimoCheck.String
		}
		if ultimoBackup.Valid {
			sw.UltimoBackup = ultimoBackup.String
		}
		switches = append(switches, sw)
	}
	return switches
}

func getSwitchByID(id int64) *SwitchNave {
	var sw SwitchNave
	var ultimoCheck, ultimoBackup sql.NullString
	err := database.DB.QueryRow(`
		SELECT id, nave_id, nome, marca, COALESCE(modello, ''), ip, ssh_port, ssh_user, ssh_pass, COALESCE(note, ''), ultimo_check, ultimo_backup
		FROM switch_nave WHERE id = ?
	`, id).Scan(&sw.ID, &sw.NaveID, &sw.Nome, &sw.Marca, &sw.Modello, &sw.IP, &sw.SSHPort, &sw.SSHUser, &sw.SSHPass, &sw.Note, &ultimoCheck, &ultimoBackup)
	if err != nil {
		return nil
	}
	if ultimoCheck.Valid {
		sw.UltimoCheck = ultimoCheck.String
	}
	if ultimoBackup.Valid {
		sw.UltimoBackup = ultimoBackup.String
	}
	return &sw
}

func getAccessPointsByNave(naveID int64) []AccessPoint {
	var aps []AccessPoint
	rows, err := database.DB.Query(`
		SELECT ap.id, ap.nave_id, ap.ac_id, ap.ap_name, ap.ap_mac, COALESCE(ap.ap_model, ''),
		       COALESCE(ap.ap_serial, ''), COALESCE(ap.ap_ip, ''), ap.switch_id, COALESCE(ap.switch_port, ''),
		       ap.stato, ap.ultimo_check, COALESCE(sw.nome, '')
		FROM access_point ap
		LEFT JOIN switch_nave sw ON ap.switch_id = sw.id
		WHERE ap.nave_id = ?
		ORDER BY ap.stato DESC, ap.ap_name
	`, naveID)
	if err != nil {
		return aps
	}
	defer rows.Close()

	for rows.Next() {
		var ap AccessPoint
		var switchID sql.NullInt64
		var ultimoCheck sql.NullString
		rows.Scan(&ap.ID, &ap.NaveID, &ap.ACID, &ap.APName, &ap.APMAC, &ap.APModel, &ap.APSerial, &ap.APIP,
			&switchID, &ap.SwitchPort, &ap.Stato, &ultimoCheck, &ap.SwitchNome)
		if switchID.Valid {
			ap.SwitchID = &switchID.Int64
		}
		if ultimoCheck.Valid {
			ap.UltimoCheck = ultimoCheck.String
		}
		aps = append(aps, ap)
	}
	return aps
}

func getRecentBackups(naveID int64, limit int) []ConfigBackup {
	var backups []ConfigBackup
	rows, err := database.DB.Query(`
		SELECT id, nave_id, tipo_apparato, apparato_id, nome_apparato, file_path, file_size, hash_md5, created_at
		FROM config_backup WHERE nave_id = ? ORDER BY created_at DESC LIMIT ?
	`, naveID, limit)
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

func countAPByStatus(naveID int64, stato string) int {
	var count int
	database.DB.QueryRow("SELECT COUNT(*) FROM access_point WHERE nave_id = ? AND stato = ?", naveID, stato).Scan(&count)
	return count
}

// parseHuaweiAPOutput parsa l'output di "display ap all" Huawei
func parseHuaweiAPOutput(output string) []map[string]string {
	var aps []map[string]string

	lines := strings.Split(output, "\n")
	// Esempio output Huawei:
	// AP ID  AP Name          State   IP Address      MAC Address        Model
	// 0      AP-Deck1         Run     192.168.1.10    00:e0:fc:12:34:56  AP6050DN

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "AP ID") || strings.HasPrefix(line, "-") {
			continue
		}

		// Usa regex per parsare (più robusto)
		// Pattern: ID Name State IP MAC Model
		re := regexp.MustCompile(`^(\d+)\s+(\S+)\s+(\S+)\s+(\d+\.\d+\.\d+\.\d+)?\s*([0-9a-fA-F:-]+)\s+(\S+)?`)
		matches := re.FindStringSubmatch(line)
		if len(matches) >= 6 {
			ap := map[string]string{
				"name":   matches[2],
				"state":  matches[3],
				"ip":     matches[4],
				"mac":    normalizeMAC(matches[5]),
				"model":  matches[6],
			}
			aps = append(aps, ap)
		}
	}

	return aps
}

// parseHuaweiMacTable parsa l'output di "display mac-address" Huawei
func parseHuaweiMacTable(output string) []map[string]string {
	var entries []map[string]string

	lines := strings.Split(output, "\n")
	// Esempio output:
	// MAC Address      VLAN  State  Port
	// 00e0-fc12-3456   100   D      GE0/0/1

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "MAC Address") || strings.HasPrefix(line, "-") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 4 {
			entry := map[string]string{
				"mac":  normalizeMAC(fields[0]),
				"vlan": fields[1],
				"port": fields[3],
			}
			entries = append(entries, entry)
		}
	}

	return entries
}

// parseHPMacTable parsa l'output di "show mac-address" HP
func parseHPMacTable(output string) []map[string]string {
	var entries []map[string]string

	lines := strings.Split(output, "\n")
	// HP ha formato leggermente diverso

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "MAC Address") || strings.HasPrefix(line, "-") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 3 {
			entry := map[string]string{
				"mac":  normalizeMAC(fields[0]),
				"vlan": fields[1],
				"port": fields[2],
			}
			entries = append(entries, entry)
		}
	}

	return entries
}

// normalizeMAC converte MAC in formato standard (minuscolo, con :)
func normalizeMAC(mac string) string {
	// Rimuovi separatori esistenti
	mac = strings.ReplaceAll(mac, "-", "")
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ReplaceAll(mac, ".", "")
	mac = strings.ToLower(mac)

	// Aggiungi : ogni 2 caratteri
	if len(mac) == 12 {
		return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
			mac[0:2], mac[2:4], mac[4:6], mac[6:8], mac[8:10], mac[10:12])
	}
	return mac
}

// updateOrCreateAP aggiorna o crea un AP nel database
func updateOrCreateAP(naveID, acID int64, ap map[string]string) {
	mac := ap["mac"]
	name := ap["name"]
	model := ap["model"]
	ip := ap["ip"]

	// Determina stato
	stato := "unknown"
	state := strings.ToLower(ap["state"])
	if state == "run" || state == "online" || state == "normal" {
		stato = "online"
	} else if state == "fault" || state == "error" {
		stato = "fault"
	} else if state == "offline" || state == "idle" {
		stato = "offline"
	}

	// Verifica se esiste
	var existingID int64
	err := database.DB.QueryRow("SELECT id FROM access_point WHERE nave_id = ? AND ap_mac = ?", naveID, mac).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Inserisci nuovo
		database.DB.Exec(`
			INSERT INTO access_point (nave_id, ac_id, ap_name, ap_mac, ap_model, ap_ip, stato, ultimo_check)
			VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, naveID, acID, name, mac, model, ip, stato)
	} else {
		// Aggiorna esistente
		database.DB.Exec(`
			UPDATE access_point SET ap_name = ?, ap_model = ?, ap_ip = ?, stato = ?, ultimo_check = CURRENT_TIMESTAMP
			WHERE id = ?
		`, name, model, ip, stato, existingID)

		// Se stato è cambiato, logga
		var oldStato string
		database.DB.QueryRow("SELECT stato FROM access_point WHERE id = ?", existingID).Scan(&oldStato)
		if oldStato != stato {
			database.DB.Exec(`
				INSERT INTO ap_status_log (ap_id, stato, dettaglio) VALUES (?, ?, ?)
			`, existingID, stato, fmt.Sprintf("Cambio stato: %s -> %s", oldStato, stato))
		}
	}
}

// updateAPSwitchPort aggiorna la porta dello switch per un AP dato il MAC
func updateAPSwitchPort(naveID, switchID int64, mac, port string) {
	mac = normalizeMAC(mac)
	database.DB.Exec(`
		UPDATE access_point SET switch_id = ?, switch_port = ?, updated_at = CURRENT_TIMESTAMP
		WHERE nave_id = ? AND ap_mac = ?
	`, switchID, port, naveID, mac)
}

// GetAPFaultCountForNave restituisce il numero di AP in fault per una nave (usato nei permessi)
func GetAPFaultCountForNave(naveID int64) int {
	return countAPByStatus(naveID, "fault")
}

// APIGetAPFault restituisce il conteggio AP in fault per una nave (usato nei permessi)
func APIGetAPFault(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	naveIDStr := r.URL.Query().Get("nave_id")
	naveID, _ := strconv.ParseInt(naveIDStr, 10, 64)

	count := countAPByStatus(naveID, "fault")
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count": count,
	})
}

// ============================================
// SCHEDULER MONITORAGGIO SETTIMANALE
// ============================================

// StartMonitoringScheduler avvia il job di monitoraggio settimanale
func StartMonitoringScheduler() {
	go func() {
		// Calcola prossimo lunedì alle 03:00
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			now := time.Now()
			// Esegui ogni lunedì alle 03:00
			if now.Weekday() == time.Monday && now.Hour() == 3 {
				log.Println("[Monitoring] Avvio job settimanale monitoraggio rete...")
				RunMonitoringJob()
			}
		}
	}()
	log.Println("[Monitoring] Scheduler monitoraggio settimanale avviato")
}

// RunMonitoringJob esegue il monitoraggio di tutte le navi attive
func RunMonitoringJob() {
	// Ottieni tutte le navi non ferme per lavori
	rows, err := database.DB.Query(`
		SELECT n.id, n.nome
		FROM navi n
		WHERE COALESCE(n.ferma_per_lavori, 0) = 0
	`)
	if err != nil {
		log.Printf("[Monitoring] Errore query navi: %v", err)
		return
	}
	defer rows.Close()

	type naveData struct {
		ID   int64
		Nome string
	}
	var navi []naveData

	for rows.Next() {
		var n naveData
		rows.Scan(&n.ID, &n.Nome)
		navi = append(navi, n)
	}

	log.Printf("[Monitoring] Trovate %d navi attive da monitorare", len(navi))

	for _, nave := range navi {
		log.Printf("[Monitoring] Processando nave: %s (ID: %d)", nave.Nome, nave.ID)

		// Scan AP dall'AC
		ac := getAccessControllerByNave(nave.ID)
		if ac != nil {
			log.Printf("[Monitoring] - Scanning AP da AC %s", ac.IP)
			runScanAPBatch(nave.ID, ac)

			// Backup configurazione AC
			log.Printf("[Monitoring] - Backup config AC")
			runBackupBatch(nave.ID, "ac", ac.ID)
		}

		// Scan MAC e backup per ogni switch
		switches := getSwitchesByNave(nave.ID)
		for _, sw := range switches {
			log.Printf("[Monitoring] - Scanning MAC da switch %s (%s)", sw.Nome, sw.IP)
			runScanMACBatch(nave.ID, &sw)

			// Backup configurazione switch
			log.Printf("[Monitoring] - Backup config switch %s", sw.Nome)
			runBackupBatch(nave.ID, "switch", sw.ID)
		}

		// Pausa tra navi per non sovraccaricare
		time.Sleep(5 * time.Second)
	}

	log.Println("[Monitoring] Job settimanale completato")
}

// runScanAPBatch esegue scan AP per una nave (versione batch)
func runScanAPBatch(naveID int64, ac *AccessController) {
	output, err := executeSSHCommand(ac.IP, ac.SSHPort, ac.SSHUser, ac.SSHPass, "display ap all")
	if err != nil {
		log.Printf("[Monitoring] Errore scan AP nave %d: %v", naveID, err)
		return
	}

	aps := parseHuaweiAPOutput(string(output))
	for _, ap := range aps {
		updateOrCreateAP(naveID, ac.ID, ap)
	}

	database.DB.Exec("UPDATE access_controller SET ultimo_check = CURRENT_TIMESTAMP WHERE id = ?", ac.ID)
	log.Printf("[Monitoring] Trovati %d AP per nave %d", len(aps), naveID)
}

// runScanMACBatch esegue scan MAC per uno switch (versione batch)
func runScanMACBatch(naveID int64, sw *SwitchNave) {
	var cmdStr string
	if sw.Marca == "huawei" {
		cmdStr = "display mac-address"
	} else {
		cmdStr = "show mac-address"
	}

	output, err := executeSSHCommand(sw.IP, sw.SSHPort, sw.SSHUser, sw.SSHPass, cmdStr)
	if err != nil {
		log.Printf("[Monitoring] Errore scan MAC switch %d: %v", sw.ID, err)
		return
	}

	var entries []map[string]string
	if sw.Marca == "huawei" {
		entries = parseHuaweiMacTable(string(output))
	} else {
		entries = parseHPMacTable(string(output))
	}

	for _, entry := range entries {
		updateAPSwitchPort(naveID, sw.ID, entry["mac"], entry["port"])
	}

	database.DB.Exec("UPDATE switch_nave SET ultimo_check = CURRENT_TIMESTAMP WHERE id = ?", sw.ID)
	log.Printf("[Monitoring] Trovate %d entry MAC per switch %d", len(entries), sw.ID)
}

// runBackupBatch esegue backup config per un apparato (versione batch)
func runBackupBatch(naveID int64, tipoApparato string, apparatoID int64) {
	var ip, sshUser, sshPass, nome string
	var sshPort int
	var cmdStr string

	if tipoApparato == "ac" {
		ac := getAccessControllerByNave(naveID)
		if ac == nil {
			return
		}
		ip = ac.IP
		sshUser = ac.SSHUser
		sshPass = ac.SSHPass
		sshPort = ac.SSHPort
		nome = "AC"
		cmdStr = "display current-configuration"
	} else {
		sw := getSwitchByID(apparatoID)
		if sw == nil {
			return
		}
		ip = sw.IP
		sshUser = sw.SSHUser
		sshPass = sw.SSHPass
		sshPort = sw.SSHPort
		nome = sw.Nome
		if sw.Marca == "huawei" {
			cmdStr = "display current-configuration"
		} else {
			cmdStr = "show running-config"
		}
	}

	output, err := executeSSHCommand(ip, sshPort, sshUser, sshPass, cmdStr)
	if err != nil {
		log.Printf("[Monitoring] Errore backup %s %d: %v", tipoApparato, apparatoID, err)
		return
	}

	// Calcola hash
	hash := md5.Sum([]byte(output))
	hashStr := hex.EncodeToString(hash[:])

	// Verifica se cambiata
	var lastHash string
	database.DB.QueryRow(`
		SELECT hash_md5 FROM config_backup
		WHERE nave_id = ? AND tipo_apparato = ? AND apparato_id = ?
		ORDER BY created_at DESC LIMIT 1
	`, naveID, tipoApparato, apparatoID).Scan(&lastHash)

	if lastHash == hashStr {
		log.Printf("[Monitoring] Config %s %s non cambiata", tipoApparato, nome)
		return
	}

	// Salva file
	backupDir := filepath.Join("data", "backups", fmt.Sprintf("nave_%d", naveID))
	os.MkdirAll(backupDir, 0755)

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_%s.cfg", tipoApparato, nome, timestamp)
	filePath := filepath.Join(backupDir, filename)

	err = os.WriteFile(filePath, []byte(output), 0644)
	if err != nil {
		log.Printf("[Monitoring] Errore salvataggio backup: %v", err)
		return
	}

	database.DB.Exec(`
		INSERT INTO config_backup (nave_id, tipo_apparato, apparato_id, nome_apparato, file_path, file_size, hash_md5)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, naveID, tipoApparato, apparatoID, nome, filePath, len(output), hashStr)

	// Aggiorna timestamp
	if tipoApparato == "ac" {
		database.DB.Exec("UPDATE access_controller SET ultimo_backup = CURRENT_TIMESTAMP WHERE id = ?", apparatoID)
	} else {
		database.DB.Exec("UPDATE switch_nave SET ultimo_backup = CURRENT_TIMESTAMP WHERE id = ?", apparatoID)
	}

	log.Printf("[Monitoring] Backup %s %s salvato", tipoApparato, nome)
}

// GestioneReteNave mostra la pagina gestione rete di una nave
func GestioneReteNave(w http.ResponseWriter, r *http.Request) {
	log.Printf("[RETE] Richiesta URL: %s", r.URL.Path)
	
	data := NewPageData("Gestione Rete Nave - FurvioGest", r)

	// Estrai ID nave dall'URL
	path := strings.TrimPrefix(r.URL.Path, "/navi/rete/")
	log.Printf("[RETE] Path dopo trim: %s", path)
	
	naveID, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		log.Printf("[RETE] Errore parsing ID: %v", err)
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}
	log.Printf("[RETE] NaveID: %d", naveID)

	// Carica dati nave
	nave := getNaveInfoByID(naveID)
	log.Printf("[RETE] Nave trovata: ID=%d Nome=%s", nave.ID, nave.Nome)
	
	if nave.ID == 0 {
		log.Printf("[RETE] Nave non trovata, redirect")
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	// Carica AC (può essere nil)
	ac := getAccessControllerByNave(naveID)

	// Carica switch
	switches := getSwitchesByNave(naveID)

	// Carica AP
	accessPoints := getAccessPointsByNave(naveID)

	// Carica backup recenti
	backups := getRecentBackups(naveID, 10)

	// Conta AP in fault
	apFault := countAPByStatus(naveID, "fault")

	pageData := ReteNavePageData{
		Nave:         nave,
		AC:           ac,
		Switches:     switches,
		AccessPoints: accessPoints,
		Backups:      backups,
		APFault:      apFault,
	}

	data.Data = pageData
	log.Printf("[RETE] Rendering template rete_nave.html")
	renderTemplate(w, "rete_nave.html", data)
}

// APIExportAPCSV esporta gli AP in formato CSV
func APIExportAPCSV(w http.ResponseWriter, r *http.Request) {
	naveID, _ := strconv.ParseInt(r.URL.Query().Get("nave_id"), 10, 64)
	
	// Ottieni nome nave per il filename
	var nomeNave string
	database.DB.QueryRow("SELECT nome FROM navi WHERE id = ?", naveID).Scan(&nomeNave)
	
	// Ottieni AP
	accessPoints := getAccessPointsByNave(naveID)
	
	// Imposta headers per download CSV
	filename := fmt.Sprintf("AP_%s_%s.csv", strings.ReplaceAll(nomeNave, " ", "_"), time.Now().Format("20060102"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	
	// BOM per Excel
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	
	// Header CSV
	w.Write([]byte("Stato;Nome AP;MAC Address;IP;Modello;Seriale;Switch;Porta;Ultimo Check\n"))
	
	// Dati
	for _, ap := range accessPoints {
		line := fmt.Sprintf("%s;%s;%s;%s;%s;%s;%s;%s;%s\n",
			ap.Stato,
			ap.APName,
			ap.APMAC,
			ap.APIP,
			ap.APModel,
			ap.APSerial,
			ap.SwitchNome,
			ap.SwitchPort,
			ap.UltimoCheck)
		w.Write([]byte(line))
	}
}
