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
	Protocollo   string // ssh o telnet
	Modello      string
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
	Protocollo   string // ssh o telnet
	PorteTotali  int
	PorteLibere  int
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
// executeSwitchCommand esegue comando usando il protocollo configurato (ssh o telnet)
func executeSwitchCommand(sw *SwitchNave, command string) (string, error) {
	if sw.Protocollo == "telnet" {
		return executeTelnetCommand(sw.IP, sw.SSHUser, sw.SSHPass, command)
	}
	// Default: SSH con fallback a telnet
	return executeSSHBackup(sw.IP, sw.SSHPort, sw.SSHUser, sw.SSHPass, command)
}

func executeSSHCommand(ip string, port int, user, pass, command string) (string, error) {
	expectScript := fmt.Sprintf(`
log_user 1
set timeout 180
spawn ssh -o StrictHostKeyChecking=no -o KexAlgorithms=+diffie-hellman-group14-sha1,diffie-hellman-group1-sha1,diffie-hellman-group-exchange-sha1 -o HostKeyAlgorithms=+ssh-rsa -o ConnectTimeout=30 -p %d %s@%s

expect {
    "assword:" {
        send "%s\r"
        exp_continue
    }
    ">" {
        send "%s\r"
    }
    -re {<[^>]+>} {
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
    -re {---- More ----} {
        send " "
        exp_continue
    }
    -re {\[Y/N\]} {
        send "N\r"
        exp_continue
    }
    ">" {
        send "quit\r"
    }
    -re {<[^>]+>} {
        send "quit\r"
    }
    timeout {
        send "quit\r"
    }
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
	protocollo := r.FormValue("protocollo")
	if protocollo == "" {
		protocollo = "ssh"
	}

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

	// Elimina i file di backup dal disco
	rows, _ := database.DB.Query("SELECT file_path FROM config_backup WHERE nave_id = ? AND tipo_apparato = 'ac'", naveID)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var filePath string
			rows.Scan(&filePath)
			os.Remove(filePath)
		}
	}
	// Elimina i record di backup dal database per AC
	database.DB.Exec("DELETE FROM config_backup WHERE nave_id = ? AND tipo_apparato = 'ac'", naveID)

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
	protocollo := r.FormValue("protocollo")
	if protocollo == "" {
		protocollo = "ssh"
	}

	// Recupera hostname automaticamente dallo switch
	nome := getSwitchHostname(ip, sshPort, sshUser, sshPass, marca, protocollo)
	log.Printf("[NUOVO SWITCH] Hostname recuperato: %s", nome)

	_, err := database.DB.Exec(`
		INSERT INTO switch_nave (nave_id, nome, marca, modello, ip, ssh_port, ssh_user, ssh_pass, note, protocollo)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, naveID, nome, marca, modello, ip, sshPort, sshUser, sshPass, note, protocollo)

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
	// Ottieni naveID per redirect
	var naveID int64
	database.DB.QueryRow("SELECT nave_id FROM switch_nave WHERE id = ?", switchID).Scan(&naveID)

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

	_, err := database.DB.Exec(`
		UPDATE switch_nave SET marca = ?, modello = ?, ip = ?, ssh_port = ?, ssh_user = ?, ssh_pass = ?, note = ?, protocollo = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, marca, modello, ip, sshPort, sshUser, sshPass, note, protocollo, switchID)

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
	// Ottieni naveID per redirect
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

	log.Printf("[SCAN AP] Output raw: %s", string(output))
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
		cmdStr = "display lldp neighbor brief"
	} else { // hp
		cmdStr = "show mac-address"
	}

	// Esegui comando SSH usando expect
	output, err := executeSwitchCommand(sw, cmdStr)
	if err != nil {
		result["message"] = "Errore connessione SSH: " + err.Error()
		json.NewEncoder(w).Encode(result)
		return
	}

	// Parse output
	var entries []map[string]string
	if sw.Marca == "huawei" {
		entries = parseHuaweiLLDPOutput(string(output), sw.ID, sw.Nome)
	} else {
		entries = parseHPMacTable(string(output))
	}

	// Aggiorna porta degli AP in base al MAC
	for _, entry := range entries {
		mac := entry["ap_name"]
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

// APIScanPorts esegue scan delle porte dello switch per contare totali e libere
func APIScanPorts(w http.ResponseWriter, r *http.Request) {
	log.Printf("[SCAN PORTS] Chiamata ricevuta: %s", r.URL.String())
	w.Header().Set("Content-Type", "application/json")

	switchIDStr := r.URL.Query().Get("switch_id")
	switchID, _ := strconv.ParseInt(switchIDStr, 10, 64)

	result := map[string]interface{}{
		"success":      false,
		"message":      "",
		"porte_totali": 0,
		"porte_libere": 0,
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
		cmdStr = "display interface brief"
	} else { // hp
		cmdStr = "show interface brief"
	}

	// Esegui comando SSH
	output, err := executeSwitchCommand(sw, cmdStr)
	if err != nil {
		result["message"] = "Errore connessione SSH: " + err.Error()
		json.NewEncoder(w).Encode(result)
		return
	}

	// Parse output per contare porte
	log.Printf("[SCAN PORTS] Output ricevuto (len=%d): %s", len(output), output[:min(500, len(output))])
	var porteTotali, porteLibere int
	if sw.Marca == "huawei" {
		porteTotali, porteLibere = parseHuaweiPortsOutput(string(output))
	} else {
		porteTotali, porteLibere = parseHPPortsOutput(string(output))
	}
	log.Printf("[SCAN PORTS] Risultato: totali=%d, libere=%d", porteTotali, porteLibere)

	// Aggiorna database
	database.DB.Exec("UPDATE switch_nave SET porte_totali = ?, porte_libere = ?, ultimo_check = CURRENT_TIMESTAMP WHERE id = ?", porteTotali, porteLibere, switchID)

	result["success"] = true
	result["message"] = fmt.Sprintf("Porte: %d totali, %d libere", porteTotali, porteLibere)
	result["porte_totali"] = porteTotali
	result["porte_libere"] = porteLibere

	json.NewEncoder(w).Encode(result)
}

// parseHuaweiPortsOutput analizza output di display interface brief
func parseHuaweiPortsOutput(output string) (totali, libere int) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Cerca righe che iniziano con GigabitEthernet, 10GigabitEthernet, GE, XGE
		if strings.HasPrefix(line, "GigabitEthernet") || strings.HasPrefix(line, "10GigabitEthernet") || strings.HasPrefix(line, "GE") || strings.HasPrefix(line, "XGE") || strings.HasPrefix(line, "Eth") {
			totali++
			// Porta libera se stato e down o *down
			lineLower := strings.ToLower(line)
			if strings.Contains(lineLower, "down") && !strings.Contains(lineLower, "up") {
				libere++
			}
		}
	}
	return totali, libere
}

// parseHPPortsOutput analizza output di show interface brief HP
func parseHPPortsOutput(output string) (totali, libere int) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// HP usa formato diverso, cerca porte ethernet
		if strings.Contains(line, "Ethernet") || strings.HasPrefix(line, "1/") || strings.HasPrefix(line, "2/") {
			totali++
			lineLower := strings.ToLower(line)
			if strings.Contains(lineLower, "down") || strings.Contains(lineLower, "disabled") {
				libere++
			}
		}
	}
	return totali, libere
}

// APIBackupConfig esegue backup configurazione
func APIBackupConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	log.Printf("[BACKUP] Richiesta backup: tipo=%s id=%s", r.URL.Query().Get("tipo"), r.URL.Query().Get("id"))

	tipoApparato := r.URL.Query().Get("tipo") // ac o switch
	apparatoIDStr := r.URL.Query().Get("id")
	apparatoID, _ := strconv.ParseInt(apparatoIDStr, 10, 64)

	result := map[string]interface{}{
		"success": false,
		"message": "",
	}

	var ip, sshUser, sshPass, nome string
	var sshPort int
	var currentSwitch *SwitchNave
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
		currentSwitch = getSwitchByID(apparatoID)
		if currentSwitch == nil {
			result["message"] = "Switch non trovato"
			json.NewEncoder(w).Encode(result)
			return
		}
		ip = currentSwitch.IP
		sshUser = currentSwitch.SSHUser
		sshPass = currentSwitch.SSHPass
		sshPort = currentSwitch.SSHPort
		naveID = currentSwitch.NaveID
		nome = currentSwitch.Nome
		if currentSwitch.Marca == "huawei" {
			cmdStr = "display current-configuration"
		} else {
			cmdStr = "show running-config"
		}
	}

	// Esegui comando backup usando expect
	var output string
	var err error
	if currentSwitch != nil {
		output, err = executeSwitchCommand(currentSwitch, cmdStr)
	} else {
		output, err = executeSSHBackup(ip, sshPort, sshUser, sshPass, cmdStr)
	}
	log.Printf("[BACKUP] Switch %s output len: %d, err: %v", nome, len(output), err)
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
		// Estrai e aggiorna modello se non presente
		// Recupera modello se non presente
		var currentModello string
		database.DB.QueryRow("SELECT COALESCE(modello, '') FROM switch_nave WHERE id = ?", apparatoID).Scan(&currentModello)
		if currentModello == "" {
			versionOutput, _ := executeSwitchCommand(currentSwitch, "display version")
			_, model := parseVersionOutput(versionOutput, "huawei")
			if model != "" {
				database.DB.Exec("UPDATE switch_nave SET modello = ? WHERE id = ?", model, apparatoID)
			}
		}
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
	protocollo := r.URL.Query().Get("protocollo")

	if port == "" {
		if protocollo == "telnet" {
			port = "23"
		} else {
			port = "22"
		}
	}

	result := map[string]interface{}{
		"success": false,
		"message": "",
	}

	var expectScript string
	if protocollo == "telnet" {
		// Script expect per Telnet
		expectScript = fmt.Sprintf(`
set timeout 60
spawn telnet %s %s
expect {
    "*ogin*" { send "%s\r"; exp_continue }
    "*sername*" { send "%s\r"; exp_continue }
    "*assword*" { send "%s\r"; exp_continue }
    "<*>" { exit 0 }
    ">" { exit 0 }
    "Login incorrect" { exit 1 }
    "Authentication failed" { exit 1 }
    "Connection refused" { exit 2 }
    "Unable to connect" { exit 2 }
    timeout { exit 4 }
    eof { exit 5 }
}
`, ip, port, user, user, pass)
	} else {
		// Script expect per SSH
		expectScript = fmt.Sprintf(`
set timeout 60
spawn ssh -o StrictHostKeyChecking=no -o KexAlgorithms=+diffie-hellman-group14-sha1,diffie-hellman-group1-sha1,diffie-hellman-group-exchange-sha1 -o HostKeyAlgorithms=+ssh-rsa -p %s %s@%s
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
	}

	cmd := exec.Command("expect", "-c", expectScript)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	
	log.Printf("[%s TEST] IP: %s, Port: %s, User: %s, ExitCode: %v, Output: %s", 
		strings.ToUpper(protocollo), ip, port, user, cmd.ProcessState.ExitCode(), outputStr)

	exitCode := cmd.ProcessState.ExitCode()
	protoName := "SSH"
	if protocollo == "telnet" {
		protoName = "Telnet"
	}
	
	if exitCode == 0 && (strings.Contains(outputStr, "<") || strings.Contains(outputStr, ">") || strings.Contains(outputStr, "Huawei") || strings.Contains(outputStr, "HUAWEI")) {
		result["success"] = true
		result["message"] = fmt.Sprintf("Connessione %s riuscita", protoName)
	} else if exitCode == 1 || strings.Contains(outputStr, "Permission denied") || strings.Contains(outputStr, "Login incorrect") || strings.Contains(outputStr, "Authentication failed") {
		result["message"] = "Password errata o utente non valido"
	} else if exitCode == 2 || strings.Contains(outputStr, "Connection refused") || strings.Contains(outputStr, "Unable to connect") {
		result["message"] = fmt.Sprintf("Connessione rifiutata - porta %s chiusa", protoName)
	} else if exitCode == 3 || exitCode == 4 || strings.Contains(outputStr, "timed out") {
		result["message"] = "Timeout - apparato non raggiungibile"
	} else if exitCode == 5 {
		result["message"] = "Connessione chiusa inaspettatamente"
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
		SELECT id, nave_id, ip, ssh_port, ssh_user, ssh_pass, COALESCE(note, ''), ultimo_check, ultimo_backup, COALESCE(modello, '')
		FROM access_controller WHERE nave_id = ?
	`, naveID).Scan(&ac.ID, &ac.NaveID, &ac.IP, &ac.SSHPort, &ac.SSHUser, &ac.SSHPass, &ac.Note, &ultimoCheck, &ultimoBackup, &ac.Modello)
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
		SELECT id, nave_id, nome, marca, COALESCE(modello, ''), ip, ssh_port, ssh_user, ssh_pass, COALESCE(note, ''), ultimo_check, ultimo_backup, COALESCE(protocollo, 'ssh'), COALESCE(porte_totali, 0), COALESCE(porte_libere, 0)
		FROM switch_nave WHERE nave_id = ? ORDER BY nome
	`, naveID)
	if err != nil {
		return switches
	}
	defer rows.Close()

	for rows.Next() {
		var sw SwitchNave
		var ultimoCheck, ultimoBackup sql.NullString
		rows.Scan(&sw.ID, &sw.NaveID, &sw.Nome, &sw.Marca, &sw.Modello, &sw.IP, &sw.SSHPort, &sw.SSHUser, &sw.SSHPass, &sw.Note, &ultimoCheck, &ultimoBackup, &sw.Protocollo, &sw.PorteTotali, &sw.PorteLibere)
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
		SELECT id, nave_id, nome, marca, COALESCE(modello, ''), ip, ssh_port, ssh_user, ssh_pass, COALESCE(note, ''), ultimo_check, ultimo_backup, COALESCE(protocollo, 'ssh'), COALESCE(porte_totali, 0), COALESCE(porte_libere, 0)
		FROM switch_nave WHERE id = ?
	`, id).Scan(&sw.ID, &sw.NaveID, &sw.Nome, &sw.Marca, &sw.Modello, &sw.IP, &sw.SSHPort, &sw.SSHUser, &sw.SSHPass, &sw.Note, &ultimoCheck, &ultimoBackup, &sw.Protocollo, &sw.PorteTotali, &sw.PorteLibere)
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
	// Formato reale output Huawei:
	// ID    MAC            Name           Group   IP           Type             State  STA  Uptime...
	// 0     484c-2911-cb30 AP-02          default 10.101.3.102 AirEngine5761-11 nor    2    20D:6H...

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Salta righe vuote, header e separatori
		if line == "" || strings.HasPrefix(line, "ID") || strings.HasPrefix(line, "-") || 
		   strings.HasPrefix(line, "Total") || strings.Contains(line, "idle  :") || 
		   strings.Contains(line, "nor   :") || strings.Contains(line, "ExtraInfo") {
			continue
		}

		// Usa regex per il formato: ID MAC Name Group IP Type State STA Uptime...
		// MAC formato: xxxx-xxxx-xxxx
		re := regexp.MustCompile(`^(\d+)\s+([0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4})\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+`)
		matches := re.FindStringSubmatch(line)
		if len(matches) >= 8 {
			ip := matches[5]
			if ip == "-" {
				ip = ""
			}
			ap := map[string]string{
				"name":   matches[3],
				"mac":    normalizeMAC(matches[2]),
				"ip":     ip,
				"model":  matches[6],
				"state":  matches[7],
			}
			aps = append(aps, ap)
		}
	}

	return aps
}

// parseHuaweiMacTable parsa l'output di "display lldp neighbor brief" Huawei
func parseHuaweiMacTable(output string) []map[string]string {
	var entries []map[string]string

	lines := strings.Split(output, "\n")
	// Formato reale output Huawei:
	// MAC            VLAN/VSI/BD   Port        Type
	// 0001-2e7a-df1e 1/-/-         GE0/0/9     dynamic

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Salta righe vuote, header e info
		if line == "" || strings.Contains(line, "MAC") || strings.HasPrefix(line, "-") ||
		   strings.HasPrefix(line, "Total") || strings.Contains(line, "Info:") ||
		   strings.Contains(line, "spawn") || strings.Contains(line, "assword") {
			continue
		}

		// Usa regex per estrarre MAC (formato xxxx-xxxx-xxxx) e porta (GE/XGE)
		re := regexp.MustCompile(`^([0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4})\s+(\S+)\s+((?:GE|XGE|Eth)[0-9/]+)\s+(\S+)`)
		matches := re.FindStringSubmatch(line)
		if len(matches) >= 5 {
			entry := map[string]string{
				"mac":  normalizeMAC(matches[1]),
				"vlan": matches[2],
				"port": matches[3],
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
	if state == "run" || state == "online" || state == "normal" || state == "nor" {
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

			// Gestione automatica guasti
			if stato == "fault" && oldStato != "fault" {
				CreaGuastoAPFault(naveID, existingID, name)
			} else if oldStato == "fault" && stato != "fault" {
				ChiudiGuastoAPFault(naveID, existingID)
			}
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

// updateAPSwitchPortByName aggiorna la porta dello switch per un AP dato il nome
func updateAPSwitchPortByName(naveID, switchID int64, apName, port string) {
	database.DB.Exec(`
		UPDATE access_point SET switch_id = ?, switch_port = ?, updated_at = CURRENT_TIMESTAMP
		WHERE nave_id = ? AND ap_name = ?
	`, switchID, port, naveID, apName)
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

	// Recupera AP in fault e offline
	rows, err := database.DB.Query(`
		SELECT id, ap_name, ap_mac, ap_ip, stato 
		FROM access_point 
		WHERE nave_id = ? AND (stato = 'fault' OR stato = 'offline')
		ORDER BY ap_name
	`, naveID)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"count": 0, "ap_list": []interface{}{}})
		return
	}
	defer rows.Close()

	type APInfo struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		MAC    string `json:"mac"`
		IP     string `json:"ip"`
		Status string `json:"status"`
	}

	var apList []APInfo
	for rows.Next() {
		var ap APInfo
		rows.Scan(&ap.ID, &ap.Name, &ap.MAC, &ap.IP, &ap.Status)
		apList = append(apList, ap)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"count":   len(apList),
		"ap_list": apList,
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


	// Backup Uffici
	log.Println("[Monitoring] Backup uffici...")
	runBackupUffici()

	// Backup Sale Server
	log.Println("[Monitoring] Backup sale server...")
	runBackupSaleServer()
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
		cmdStr = "display lldp neighbor brief"
	} else {
		cmdStr = "show mac-address"
	}

	output, err := executeSwitchCommand(sw, cmdStr)
	if err != nil {
		log.Printf("[Monitoring] Errore scan MAC switch %d: %v", sw.ID, err)
		return
	}

	var entries []map[string]string
	if sw.Marca == "huawei" {
		entries = parseHuaweiLLDPOutput(string(output), sw.ID, sw.Nome)
	} else {
		entries = parseHPMacTable(string(output))
	}

	for _, entry := range entries {
		updateAPSwitchPortByName(naveID, sw.ID, entry["ap_name"], entry["port"])
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

	output, err := executeSSHBackup(ip, sshPort, sshUser, sshPass, cmdStr)
	log.Printf("[BACKUP] Switch %s output len: %d, err: %v", nome, len(output), err)
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
		// Estrai e aggiorna modello se non presente
		// Recupera modello se non presente
		var currentModello string
		database.DB.QueryRow("SELECT COALESCE(modello, '') FROM switch_nave WHERE id = ?", apparatoID).Scan(&currentModello)
		if currentModello == "" {
			versionOutput, _ := executeSSHBackup(ip, sshPort, sshUser, sshPass, "display version")
			_, model := parseVersionOutput(versionOutput, "huawei")
			if model != "" {
				database.DB.Exec("UPDATE switch_nave SET modello = ? WHERE id = ?", model, apparatoID)
			}
		}
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

// APIScanLLDP esegue scan LLDP su tutti gli switch della nave e associa gli AP alle porte
func APIScanLLDP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	naveIDStr := r.URL.Query().Get("nave_id")
	naveID, _ := strconv.ParseInt(naveIDStr, 10, 64)

	result := map[string]interface{}{
		"success":    false,
		"message":    "",
		"ap_trovati": 0,
		"dettagli":   []map[string]string{},
	}

	// Verifica nave
	nave := getNaveInfoByID(naveID)
	if nave.ID == 0 {
		result["message"] = "Nave non trovata"
		json.NewEncoder(w).Encode(result)
		return
	}

	// Ottieni tutti gli switch della nave
	switches := getSwitchesByNave(naveID)
	if len(switches) == 0 {
		result["message"] = "Nessuno switch configurato per questa nave"
		json.NewEncoder(w).Encode(result)
		return
	}

	var totalEntries int
	var errors []string

	for _, sw := range switches {
		// Esegui comando LLDP
		output, err := executeSSHBackup(sw.IP, sw.SSHPort, sw.SSHUser, sw.SSHPass, "display lldp neighbor brief")
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", sw.Nome, err))
			continue
		}

		// Parse output LLDP
		entries := parseHuaweiLLDPOutput(string(output), sw.ID, sw.Nome)
		for _, entry := range entries {
			updateAPSwitchPortByName(naveID, sw.ID, entry["ap_name"], entry["port"])
		}
		totalEntries += len(entries)
		log.Printf("[LLDP] Switch %s: trovati %d AP", sw.Nome, len(entries))

		// Aggiorna timestamp switch
		database.DB.Exec("UPDATE switch_nave SET ultimo_check = CURRENT_TIMESTAMP WHERE id = ?", sw.ID)
	}


	result["success"] = len(errors) == 0 || totalEntries > 0
	result["ap_trovati"] = totalEntries
	result["dettagli"] = []map[string]string{}

	if len(errors) > 0 {
		result["message"] = fmt.Sprintf("Trovati %d AP. Errori su: %s", totalEntries, strings.Join(errors, ", "))
	} else {
		result["message"] = fmt.Sprintf("Scan LLDP completato. Trovati %d AP su %d switch", totalEntries, len(switches))
	}

	json.NewEncoder(w).Encode(result)
}

// parseHuaweiLLDPOutput parsa output "display lldp neighbor brief" e estrae gli AP
func parseHuaweiLLDPOutput(output string, switchID int64, switchNome string) []map[string]string {
	var aps []map[string]string

	lines := strings.Split(output, "\n")
	// Formato:
	// Local Intf       Neighbor Dev             Neighbor Intf             Exptime(s)
	// GE0/0/17         AP-06                    GE0/0/0                   118

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Salta header e righe vuote
		if line == "" || strings.HasPrefix(line, "Local") || strings.Contains(line, "spawn") ||
			strings.Contains(line, "assword") || strings.Contains(line, "Info:") {
			continue
		}

		// Parse: porta locale, nome neighbor, porta neighbor, expire
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			localPort := fields[0]
			neighborName := fields[1]

			// Filtra solo AP (nome inizia con AP-)
			if strings.HasPrefix(neighborName, "AP-") || strings.HasPrefix(neighborName, "AP_") {
				ap := map[string]string{
					"ap_name":     neighborName,
					"port":        localPort,
					"switch_id":   fmt.Sprintf("%d", switchID),
					"switch_nome": switchNome,
				}
				aps = append(aps, ap)
			}
		}
	}

	return aps
}

// APIGetSwitchVersion ottiene la versione firmware di uno switch
func APIGetSwitchVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switchIDStr := r.URL.Query().Get("switch_id")
	switchID, _ := strconv.ParseInt(switchIDStr, 10, 64)

	result := map[string]interface{}{
		"success": false,
		"message": "",
		"version": "",
		"model":   "",
	}

	sw := getSwitchByID(switchID)
	if sw == nil {
		result["message"] = "Switch non trovato"
		json.NewEncoder(w).Encode(result)
		return
	}

	// Comando per ottenere versione
	var cmdStr string
	if sw.Marca == "huawei" {
		cmdStr = "display version"
	} else {
		cmdStr = "show version"
	}

	output, err := executeSwitchCommand(sw, cmdStr)
	if err != nil {
		result["message"] = "Errore connessione: " + err.Error()
		json.NewEncoder(w).Encode(result)
		return
	}

	// Parse versione
	version, model := parseVersionOutput(string(output), sw.Marca)

	// Aggiorna modello nel DB se trovato
	if model != "" && sw.Modello == "" {
		database.DB.Exec("UPDATE switch_nave SET modello = ? WHERE id = ?", model, switchID)
	}

	result["success"] = true
	result["version"] = version
	result["model"] = model
	result["message"] = "Versione ottenuta"

	json.NewEncoder(w).Encode(result)
}

// parseVersionOutput estrae versione e modello dall output Huawei
func parseVersionOutput(output, marca string) (string, string) {
	var version, model string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if marca == "huawei" {
			// Versione: VRP (R) software, Version 5.170 (S5735 V200R021C00SPC100)
			if strings.Contains(line, "VRP") && strings.Contains(line, "Version") && !strings.HasPrefix(line, "Software") {
				re := regexp.MustCompile(`\(([A-Z0-9]+)\s+([A-Z0-9]+)\)`)
				if m := re.FindStringSubmatch(line); len(m) > 2 {
					version = m[2]
				}
			}
			// Modello: HUAWEI S5735-L24P4S-A1 Routing Switch
			if strings.Contains(line, "HUAWEI") && strings.Contains(line, "Switch") {
				re := regexp.MustCompile(`HUAWEI\s+([A-Z0-9-]+)`)
				if m := re.FindStringSubmatch(line); len(m) > 1 {
					model = m[1]
				}
			}
		} else {
			if strings.Contains(line, "Software Version") {
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					version = strings.TrimSpace(parts[1])
				}
			}
		}
	}
	return version, model
}


// APIGetACVersion ottiene la versione firmware dell AC
func APIGetACVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	acIDStr := r.URL.Query().Get("ac_id")
	acID, _ := strconv.ParseInt(acIDStr, 10, 64)

	result := map[string]interface{}{
		"success": false,
		"message": "",
		"version": "",
		"model":   "",
	}

	var ac AccessController
	err := database.DB.QueryRow(`
		SELECT id, ip, ssh_port, ssh_user, ssh_pass FROM access_controller WHERE id = ?
	`, acID).Scan(&ac.ID, &ac.IP, &ac.SSHPort, &ac.SSHUser, &ac.SSHPass)

	if err != nil {
		result["message"] = "AC non trovato"
		json.NewEncoder(w).Encode(result)
		return
	}

	output, err := executeSSHCommand(ac.IP, ac.SSHPort, ac.SSHUser, ac.SSHPass, "display version")
	if err != nil {
		result["message"] = "Errore connessione: " + err.Error()
		json.NewEncoder(w).Encode(result)
		return
	}

	version, model := parseACVersionOutput(string(output))

	if model != "" {
		database.DB.Exec("UPDATE access_controller SET modello = ? WHERE id = ?", model, acID)
	}

	result["success"] = true
	result["version"] = version
	result["model"] = model
	result["message"] = "Versione ottenuta"

	json.NewEncoder(w).Encode(result)
}

// parseACVersionOutput estrae versione e modello dall output AC Huawei
func parseACVersionOutput(output string) (string, string) {
	var version, model string

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// VRP (R) software, Version 5.170 (AC6508 V200R019C10SPC800)
		if strings.Contains(line, "VRP") && strings.Contains(line, "Version") {
			re := regexp.MustCompile(`Version\s+(\S+)`)
			if m := re.FindStringSubmatch(line); len(m) > 1 {
				version = m[1]
			}
		}
		// Huawei AC6508 Wireless Access Controller
		if strings.Contains(line, "Huawei") && (strings.Contains(line, "AC") || strings.Contains(line, "Controller")) {
			re := regexp.MustCompile(`Huawei\s+(\S+)`)
			if m := re.FindStringSubmatch(line); len(m) > 1 {
				model = m[1]
			}
		}
	}

	return version, model
}

// parseVersionOutput estrae versione e modello dall output Huawei

// executeSSHBackup esegue comando SSH con paginazione disabilitata per backup lunghi
func executeSSHBackup(ip string, port int, user, pass, command string) (string, error) {
	expectScript := fmt.Sprintf(`
log_user 1
set timeout 300
spawn ssh -o StrictHostKeyChecking=no -o KexAlgorithms=+diffie-hellman-group14-sha1,diffie-hellman-group1-sha1,diffie-hellman-group-exchange-sha1 -o HostKeyAlgorithms=+ssh-rsa -o ConnectTimeout=30 -p %d %s@%s

expect {
    "assword:" {
        send "%s\r"
        exp_continue
    }
    ">" {
    }
    -re {<[^>]+>} {
    }
    timeout {
        exit 4
    }
    "Permission denied" {
        exit 1
    }
}

# Disabilita paginazione
send "screen-length 0 temporary\r"
expect {
    ">" {}
    -re {<[^>]+>} {}
    timeout {}
}
sleep 1

# Esegui comando backup
send "%s\r"
expect {
    ">" {}
    -re {<[^>]+>} {}
    timeout {}
}

send "quit\r"
expect eof
`, port, user, ip, pass, command)

	cmd := exec.Command("expect", "-c", expectScript)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	// Se SSH fallisce con shell request failed, prova telnet
	if err != nil || strings.Contains(output, "shell request failed") {
		log.Printf("[SSH] Fallback a telnet per %s", ip)
		return executeTelnetCommand(ip, user, pass, command)
	}
	return output, nil
}

// executeTelnetCommand esegue comando via Telnet (fallback per switch senza SSH)
func executeTelnetCommand(ip string, user, pass, command string) (string, error) {
	expectScript := fmt.Sprintf(`
log_user 1
set timeout 60
spawn telnet %s
expect {
    "Username:" { send "%s\r" }
    "login:" { send "%s\r" }
    timeout { exit 4 }
}
expect "*assword*" { send "%s\r" }
expect {
    ">" {}
    -re {<[^>]+>} {}
    timeout { exit 4 }
}

# Disabilita paginazione
send "screen-length 0 temporary\r"
expect {
    ">" {}
    -re {<[^>]+>} {}
}
sleep 1

send "%s\r"
expect {
    ">" {}
    -re {<[^>]+>} {}
    timeout {}
}

send "quit\r"
expect eof
`, ip, user, user, pass, command)

	cmd := exec.Command("expect", "-c", expectScript)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("Telnet error: %v - %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// getSwitchHostname recupera l'hostname dello switch via SSH/Telnet
func getSwitchHostname(ip string, port int, user, pass, marca, protocollo string) string {
	var cmd string
	if marca == "huawei" {
		cmd = "display current-configuration | include sysname"
	} else {
		cmd = "show running-config | include hostname"
	}

	var output string
	var err error
	if protocollo == "telnet" {
		output, err = executeTelnetCommand(ip, user, pass, cmd)
	} else {
		output, err = executeSSHCommand(ip, port, user, pass, cmd)
	}

	if err != nil {
		log.Printf("[HOSTNAME] Errore recupero hostname: %v", err)
		return "Switch-" + ip // Fallback: usa IP come nome
	}

	// Parse output per estrarre hostname
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Huawei: sysname SW-PONTE-1
		if strings.HasPrefix(strings.ToLower(line), "sysname ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "sysname "))
		}
		// HP: hostname SW-PONTE-1
		if strings.HasPrefix(strings.ToLower(line), "hostname ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "hostname "))
		}
	}

	// Fallback: usa IP
	return "Switch-" + ip
}

// runBackupUffici esegue backup di tutti gli uffici
func runBackupUffici() {
	// Ottieni tutti gli uffici
	rows, err := database.DB.Query("SELECT id, nome FROM uffici")
	if err != nil {
		log.Printf("[Monitoring] Errore query uffici: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var nome string
		rows.Scan(&id, &nome)

		log.Printf("[Monitoring] Backup ufficio: %s", nome)

		// Backup AC ufficio
		var acID int64
		err := database.DB.QueryRow("SELECT id FROM ac_ufficio WHERE ufficio_id = ?", id).Scan(&acID)
		if err == nil {
			runBackupUfficioBatch(id, 0, "ac", acID)
		}

		// Backup switch ufficio
		swRows, _ := database.DB.Query("SELECT id, nome FROM switch_ufficio WHERE ufficio_id = ?", id)
		if swRows != nil {
			for swRows.Next() {
				var swID int64
				var swNome string
				swRows.Scan(&swID, &swNome)
				log.Printf("[Monitoring] - Backup switch %s", swNome)
				runBackupUfficioBatch(id, 0, "switch", swID)
			}
			swRows.Close()
		}

		time.Sleep(2 * time.Second)
	}
}

// runBackupSaleServer esegue backup di tutte le sale server
func runBackupSaleServer() {
	// Ottieni tutte le sale server
	rows, err := database.DB.Query("SELECT id, nome FROM sale_server")
	if err != nil {
		log.Printf("[Monitoring] Errore query sale server: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var nome string
		rows.Scan(&id, &nome)

		log.Printf("[Monitoring] Backup sala server: %s", nome)

		// Backup switch sala server
		swRows, _ := database.DB.Query("SELECT id, nome FROM switch_sala_server WHERE sala_server_id = ?", id)
		if swRows != nil {
			for swRows.Next() {
				var swID int64
				var swNome string
				swRows.Scan(&swID, &swNome)
				log.Printf("[Monitoring] - Backup switch %s", swNome)
				runBackupUfficioBatch(0, id, "switch", swID)
			}
			swRows.Close()
		}

		time.Sleep(2 * time.Second)
	}
}

// runBackupUfficioBatch esegue backup per un apparato ufficio/sala server
func runBackupUfficioBatch(ufficioID, salaServerID int64, tipo string, apparatoID int64) {
	var ip, sshUser, sshPass, nome, protocollo string
	var sshPort int
	var cmdStr string
	var tabella string

	if ufficioID > 0 {
		if tipo == "ac" {
			tabella = "ac_ufficio"
			err := database.DB.QueryRow("SELECT ip, ssh_port, ssh_user, ssh_pass, COALESCE(protocollo, 'ssh'), COALESCE(porte_totali, 0), COALESCE(porte_libere, 0) FROM ac_ufficio WHERE id = ?", apparatoID).
				Scan(&ip, &sshPort, &sshUser, &sshPass, &protocollo)
			if err != nil {
				return
			}
			nome = "AC"
			cmdStr = "display current-configuration"
		} else {
			tabella = "switch_ufficio"
			var marca string
			err := database.DB.QueryRow("SELECT nome, ip, ssh_port, ssh_user, ssh_pass, COALESCE(protocollo, 'ssh'), COALESCE(porte_totali, 0), COALESCE(porte_libere, 0), marca FROM switch_ufficio WHERE id = ?", apparatoID).
				Scan(&nome, &ip, &sshPort, &sshUser, &sshPass, &protocollo, &marca)
			if err != nil {
				return
			}
			if marca == "huawei" {
				cmdStr = "display current-configuration"
			} else {
				cmdStr = "show running-config"
			}
		}
	} else {
		tabella = "switch_sala_server"
		var marca string
		err := database.DB.QueryRow("SELECT nome, ip, ssh_port, ssh_user, ssh_pass, COALESCE(protocollo, 'ssh'), COALESCE(porte_totali, 0), COALESCE(porte_libere, 0), marca FROM switch_sala_server WHERE id = ?", apparatoID).
			Scan(&nome, &ip, &sshPort, &sshUser, &sshPass, &protocollo, &marca)
		if err != nil {
			return
		}
		if marca == "huawei" {
			cmdStr = "display current-configuration"
		} else {
			cmdStr = "show running-config"
		}
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
		log.Printf("[Monitoring] Errore backup %s: %v", nome, err)
		return
	}

	// Calcola hash
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
	filename := fmt.Sprintf("%s_%s_%s.cfg", tipo, nome, timestamp)
	filePath := filepath.Join(backupDir, filename)

	err = os.WriteFile(filePath, []byte(output), 0644)
	if err != nil {
		log.Printf("[Monitoring] Errore salvataggio backup %s: %v", nome, err)
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

	database.DB.Exec("INSERT INTO config_backup_ufficio (ufficio_id, sala_server_id, tipo_apparato, apparato_id, nome_apparato, file_path, file_size, hash_md5) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		ufficioIDPtr, salaServerIDPtr, tipo, apparatoID, nome, filePath, len(output), hashStr)

	// Aggiorna timestamp
	database.DB.Exec(fmt.Sprintf("UPDATE %s SET ultimo_backup = CURRENT_TIMESTAMP WHERE id = ?", tabella), apparatoID)

	log.Printf("[Monitoring] Backup %s salvato", nome)
}
