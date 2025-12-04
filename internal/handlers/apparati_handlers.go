package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"furviogest/internal/database"
	"furviogest/internal/models"
)

// ============================================
// GESTIONE APPARATI NAVE
// ============================================

type ApparatiPageData struct {
	Nave      models.Nave
	Apparati  []models.ApparatoNave
	Tipi      []models.TipoApparato
	Apparato  models.ApparatoNave
}

// ListaApparati mostra gli apparati di una nave
func ListaApparati(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Apparati Nave - FurvioGest", r)

	// Estrai ID nave dall'URL
	path := strings.TrimPrefix(r.URL.Path, "/navi/apparati/")
	naveID, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	// Carica dati nave
	nave := getNaveByID(naveID)
	if nave.ID == 0 {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	// Carica apparati
	apparati := getApparatiNave(naveID)
	tipi := getTipiApparato()

	pageData := ApparatiPageData{
		Nave:     nave,
		Apparati: apparati,
		Tipi:     tipi,
	}

	data.Data = pageData
	renderTemplate(w, "apparati_lista.html", data)
}

// NuovoApparato gestisce creazione apparato
func NuovoApparato(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Nuovo Apparato - FurvioGest", r)

	path := strings.TrimPrefix(r.URL.Path, "/navi/apparati/nuovo/")
	naveID, _ := strconv.ParseInt(path, 10, 64)

	nave := getNaveByID(naveID)
	if nave.ID == 0 {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	tipi := getTipiApparato()

	if r.Method == http.MethodGet {
		pageData := ApparatiPageData{
			Nave: nave,
			Tipi: tipi,
		}
		data.Data = pageData
		renderTemplate(w, "apparato_form.html", data)
		return
	}

	// POST - salva apparato
	r.ParseForm()
	
	sshPort, _ := strconv.Atoi(r.FormValue("ssh_port"))
	if sshPort == 0 { sshPort = 22 }
	httpPort, _ := strconv.Atoi(r.FormValue("http_port"))
	if httpPort == 0 { httpPort = 80 }
	httpsEnabled := r.FormValue("https_enabled") == "on"

	_, err := database.DB.Exec(`
		INSERT INTO apparati_nave (nave_id, nome, tipo, ip, mac, vendor, modello, firmware,
			location, snmp_community, ssh_user, ssh_pass, ssh_port, http_user, http_pass,
			http_port, https_enabled, note, stato)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'unknown')
	`,
		naveID,
		strings.TrimSpace(r.FormValue("nome")),
		r.FormValue("tipo"),
		strings.TrimSpace(r.FormValue("ip")),
		strings.TrimSpace(r.FormValue("mac")),
		strings.TrimSpace(r.FormValue("vendor")),
		strings.TrimSpace(r.FormValue("modello")),
		strings.TrimSpace(r.FormValue("firmware")),
		strings.TrimSpace(r.FormValue("location")),
		strings.TrimSpace(r.FormValue("snmp_community")),
		strings.TrimSpace(r.FormValue("ssh_user")),
		strings.TrimSpace(r.FormValue("ssh_pass")),
		sshPort,
		strings.TrimSpace(r.FormValue("http_user")),
		strings.TrimSpace(r.FormValue("http_pass")),
		httpPort,
		httpsEnabled,
		strings.TrimSpace(r.FormValue("note")),
	)

	if err != nil {
		data.Error = "Errore nel salvataggio: " + err.Error()
		pageData := ApparatiPageData{Nave: nave, Tipi: tipi}
		data.Data = pageData
		renderTemplate(w, "apparato_form.html", data)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/navi/apparati/%d", naveID), http.StatusSeeOther)
}

// ModificaApparato gestisce modifica apparato
func ModificaApparato(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Modifica Apparato - FurvioGest", r)

	path := strings.TrimPrefix(r.URL.Path, "/navi/apparato/modifica/")
	apparatoID, _ := strconv.ParseInt(path, 10, 64)

	apparato := getApparatoByID(apparatoID)
	if apparato.ID == 0 {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	nave := getNaveByID(apparato.NaveID)
	tipi := getTipiApparato()

	if r.Method == http.MethodGet {
		pageData := ApparatiPageData{
			Nave:     nave,
			Apparato: apparato,
			Tipi:     tipi,
		}
		data.Data = pageData
		renderTemplate(w, "apparato_form.html", data)
		return
	}

	// POST - aggiorna
	r.ParseForm()
	
	sshPort, _ := strconv.Atoi(r.FormValue("ssh_port"))
	if sshPort == 0 { sshPort = 22 }
	httpPort, _ := strconv.Atoi(r.FormValue("http_port"))
	if httpPort == 0 { httpPort = 80 }
	httpsEnabled := r.FormValue("https_enabled") == "on"

	_, err := database.DB.Exec(`
		UPDATE apparati_nave SET
			nome = ?, tipo = ?, ip = ?, mac = ?, vendor = ?, modello = ?, firmware = ?,
			location = ?, snmp_community = ?, ssh_user = ?, ssh_pass = ?, ssh_port = ?,
			http_user = ?, http_pass = ?, http_port = ?, https_enabled = ?, note = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`,
		strings.TrimSpace(r.FormValue("nome")),
		r.FormValue("tipo"),
		strings.TrimSpace(r.FormValue("ip")),
		strings.TrimSpace(r.FormValue("mac")),
		strings.TrimSpace(r.FormValue("vendor")),
		strings.TrimSpace(r.FormValue("modello")),
		strings.TrimSpace(r.FormValue("firmware")),
		strings.TrimSpace(r.FormValue("location")),
		strings.TrimSpace(r.FormValue("snmp_community")),
		strings.TrimSpace(r.FormValue("ssh_user")),
		strings.TrimSpace(r.FormValue("ssh_pass")),
		sshPort,
		strings.TrimSpace(r.FormValue("http_user")),
		strings.TrimSpace(r.FormValue("http_pass")),
		httpPort,
		httpsEnabled,
		strings.TrimSpace(r.FormValue("note")),
		apparatoID,
	)

	if err != nil {
		data.Error = "Errore nel salvataggio"
		pageData := ApparatiPageData{Nave: nave, Apparato: apparato, Tipi: tipi}
		data.Data = pageData
		renderTemplate(w, "apparato_form.html", data)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/navi/apparati/%d", apparato.NaveID), http.StatusSeeOther)
}

// EliminaApparato elimina un apparato
func EliminaApparato(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/navi/apparato/elimina/")
	apparatoID, _ := strconv.ParseInt(path, 10, 64)

	apparato := getApparatoByID(apparatoID)
	naveID := apparato.NaveID

	database.DB.Exec("DELETE FROM apparati_nave WHERE id = ?", apparatoID)
	http.Redirect(w, r, fmt.Sprintf("/navi/apparati/%d", naveID), http.StatusSeeOther)
}

// ============================================
// CONFIGURAZIONE OBSERVIUM
// ============================================

// ConfigObservium gestisce la configurazione Observium della nave
func ConfigObservium(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Configurazione Observium - FurvioGest", r)

	path := strings.TrimPrefix(r.URL.Path, "/navi/observium/")
	naveID, _ := strconv.ParseInt(path, 10, 64)

	nave := getNaveByID(naveID)
	if nave.ID == 0 {
		http.Redirect(w, r, "/navi", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		data.Data = nave
		renderTemplate(w, "observium_config.html", data)
		return
	}

	// POST - salva configurazione
	r.ParseForm()
	sshPort, _ := strconv.Atoi(r.FormValue("observium_ssh_port"))
	if sshPort == 0 { sshPort = 22 }

	_, err := database.DB.Exec(`
		UPDATE navi SET
			observium_ip = ?,
			observium_user = ?,
			observium_pass = ?,
			observium_ssh_user = ?,
			observium_ssh_pass = ?,
			observium_ssh_port = ?,
			snmp_community = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`,
		strings.TrimSpace(r.FormValue("observium_ip")),
		strings.TrimSpace(r.FormValue("observium_user")),
		strings.TrimSpace(r.FormValue("observium_pass")),
		strings.TrimSpace(r.FormValue("observium_ssh_user")),
		strings.TrimSpace(r.FormValue("observium_ssh_pass")),
		sshPort,
		strings.TrimSpace(r.FormValue("snmp_community")),
		naveID,
	)

	if err != nil {
		data.Error = "Errore nel salvataggio"
	} else {
		data.Success = "Configurazione salvata con successo"
	}

	nave = getNaveByID(naveID) // Ricarica
	data.Data = nave
	renderTemplate(w, "observium_config.html", data)
}

// ============================================
// API TEST E DISCOVERY
// ============================================

// APITestConnection testa la connessione a Observium/IP
func APITestConnection(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ip := r.URL.Query().Get("ip")
	testType := r.URL.Query().Get("type") // ping, http, ssh

	result := map[string]interface{}{
		"success": false,
		"message": "",
		"latency": 0,
	}

	if ip == "" {
		result["message"] = "IP non specificato"
		json.NewEncoder(w).Encode(result)
		return
	}

	switch testType {
	case "ping":
		// Test ICMP ping
		start := time.Now()
		cmd := exec.Command("ping", "-c", "1", "-W", "2", ip)
		err := cmd.Run()
		latency := time.Since(start).Milliseconds()
		
		if err == nil {
			result["success"] = true
			result["message"] = fmt.Sprintf("Host raggiungibile (%dms)", latency)
			result["latency"] = latency
		} else {
			result["message"] = "Host non raggiungibile"
		}

	case "http":
		port := r.URL.Query().Get("port")
		if port == "" { port = "80" }
		
		start := time.Now()
		conn, err := net.DialTimeout("tcp", ip+":"+port, 3*time.Second)
		latency := time.Since(start).Milliseconds()
		
		if err == nil {
			conn.Close()
			result["success"] = true
			result["message"] = fmt.Sprintf("Porta %s aperta (%dms)", port, latency)
			result["latency"] = latency
		} else {
			result["message"] = fmt.Sprintf("Porta %s chiusa o non raggiungibile", port)
		}

	case "ssh":
		port := r.URL.Query().Get("port")
		if port == "" { port = "22" }
		
		start := time.Now()
		conn, err := net.DialTimeout("tcp", ip+":"+port, 3*time.Second)
		latency := time.Since(start).Milliseconds()
		
		if err == nil {
			conn.Close()
			result["success"] = true
			result["message"] = fmt.Sprintf("SSH raggiungibile (%dms)", latency)
			result["latency"] = latency
		} else {
			result["message"] = "SSH non raggiungibile"
		}

	default:
		result["message"] = "Tipo test non valido"
	}

	json.NewEncoder(w).Encode(result)
}

// APIDiscoveryDevices esegue discovery dei dispositivi
func APIDiscoveryDevices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	naveIDStr := r.URL.Query().Get("nave_id")
	naveID, _ := strconv.ParseInt(naveIDStr, 10, 64)

	result := map[string]interface{}{
		"success": false,
		"message": "",
		"devices": []models.DeviceDiscovery{},
	}

	if naveID == 0 {
		result["message"] = "Nave non specificata"
		json.NewEncoder(w).Encode(result)
		return
	}

	nave := getNaveByID(naveID)
	if nave.ObserviumIP == "" {
		result["message"] = "Observium non configurato per questa nave"
		json.NewEncoder(w).Encode(result)
		return
	}

	// Prova a connettersi via SSH a Observium per ottenere lista dispositivi
	devices, err := discoverDevicesFromObservium(nave)
	if err != nil {
		result["message"] = "Errore discovery: " + err.Error()
		json.NewEncoder(w).Encode(result)
		return
	}

	result["success"] = true
	result["message"] = fmt.Sprintf("Trovati %d dispositivi", len(devices))
	result["devices"] = devices
	json.NewEncoder(w).Encode(result)
}

// APIImportDevice importa un dispositivo scoperto
func APIImportDevice(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	result := map[string]interface{}{
		"success": false,
		"message": "",
	}

	if r.Method != http.MethodPost {
		result["message"] = "Metodo non consentito"
		json.NewEncoder(w).Encode(result)
		return
	}

	r.ParseForm()
	naveID, _ := strconv.ParseInt(r.FormValue("nave_id"), 10, 64)
	
	_, err := database.DB.Exec(`
		INSERT INTO apparati_nave (nave_id, nome, tipo, ip, mac, vendor, location, stato)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'online')
	`,
		naveID,
		r.FormValue("hostname"),
		r.FormValue("type"),
		r.FormValue("ip"),
		r.FormValue("mac"),
		r.FormValue("vendor"),
		r.FormValue("location"),
	)

	if err != nil {
		result["message"] = "Errore importazione: " + err.Error()
	} else {
		result["success"] = true
		result["message"] = "Dispositivo importato"
	}

	json.NewEncoder(w).Encode(result)
}

// APICheckApparato verifica stato apparato
func APICheckApparato(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	apparatoIDStr := r.URL.Query().Get("id")
	apparatoID, _ := strconv.ParseInt(apparatoIDStr, 10, 64)

	result := map[string]interface{}{
		"success": false,
		"status":  "unknown",
		"message": "",
	}

	apparato := getApparatoByID(apparatoID)
	if apparato.ID == 0 || apparato.IP == "" {
		result["message"] = "Apparato non trovato o IP mancante"
		json.NewEncoder(w).Encode(result)
		return
	}

	// Ping test
	cmd := exec.Command("ping", "-c", "1", "-W", "2", apparato.IP)
	err := cmd.Run()

	var stato string
	if err == nil {
		stato = "online"
		result["success"] = true
		result["message"] = "Dispositivo online"
	} else {
		stato = "offline"
		result["message"] = "Dispositivo non raggiungibile"
	}
	result["status"] = stato

	// Aggiorna stato nel DB
	database.DB.Exec(`
		UPDATE apparati_nave SET stato = ?, ultimo_check = CURRENT_TIMESTAMP WHERE id = ?
	`, stato, apparatoID)

	json.NewEncoder(w).Encode(result)
}

// ============================================
// HELPER FUNCTIONS
// ============================================

func getNaveByID(id int64) models.Nave {
	var nave models.Nave
	var fermaPerLavori int
	var dataInizio, dataFine, observiumIP, observiumUser, observiumPass sql.NullString
	var observiumSSHUser, observiumSSHPass, snmpCommunity sql.NullString
	var observiumSSHPort sql.NullInt64

	err := database.DB.QueryRow(`
		SELECT n.id, n.compagnia_id, n.nome, COALESCE(n.imo, '') AS imo, COALESCE(n.email_master, '') AS email_master, COALESCE(n.email_direttore_macchina, '') AS email_direttore_macchina,
		       COALESCE(n.email_ispettore, '') AS email_ispettore, COALESCE(n.note, '') AS note, n.ferma_per_lavori, n.data_inizio_lavori,
		       n.data_fine_lavori_prevista, COALESCE(c.nome, '') AS nome_compagnia,
		       n.observium_ip, n.observium_user, n.observium_pass,
		       n.observium_ssh_user, n.observium_ssh_pass, n.observium_ssh_port, n.snmp_community
		FROM navi n
		LEFT JOIN compagnie c ON n.compagnia_id = c.id
		WHERE n.id = ?
	`, id).Scan(
		&nave.ID, &nave.CompagniaID, &nave.Nome, &nave.IMO, &nave.EmailMaster,
		&nave.EmailDirettoreMacchina, &nave.EmailIspettore, &nave.Note,
		&fermaPerLavori, &dataInizio, &dataFine, &nave.NomeCompagnia,
		&observiumIP, &observiumUser, &observiumPass,
		&observiumSSHUser, &observiumSSHPass, &observiumSSHPort, &snmpCommunity,
	)

	if err == nil {
		nave.FermaPerLavori = fermaPerLavori == 1
		if dataInizio.Valid {
			t, _ := time.Parse("2006-01-02", dataInizio.String)
			nave.DataInizioLavori = &t
		}
		if dataFine.Valid {
			t, _ := time.Parse("2006-01-02", dataFine.String)
			nave.DataFineLavoriPrev = &t
		}
		if observiumIP.Valid { nave.ObserviumIP = observiumIP.String }
		if observiumUser.Valid { nave.ObserviumUser = observiumUser.String }
		if observiumPass.Valid { nave.ObserviumPass = observiumPass.String }
		if observiumSSHUser.Valid { nave.ObserviumSSHUser = observiumSSHUser.String }
		if observiumSSHPass.Valid { nave.ObserviumSSHPass = observiumSSHPass.String }
		if observiumSSHPort.Valid { nave.ObserviumSSHPort = int(observiumSSHPort.Int64) }
		if snmpCommunity.Valid { nave.SNMPCommunity = snmpCommunity.String }
	}
	return nave
}

func getApparatiNave(naveID int64) []models.ApparatoNave {
	var apparati []models.ApparatoNave

	rows, err := database.DB.Query(`
		SELECT a.id, a.nave_id, a.nome, a.tipo, a.ip, a.mac, a.vendor, a.modello,
		       a.firmware, a.location, a.snmp_community, a.ssh_user, a.ssh_pass, a.ssh_port,
		       a.http_user, a.http_pass, a.http_port, a.https_enabled, a.note,
		       a.ultimo_check, a.stato,
		       COALESCE(t.icona, 'device'), COALESCE(t.colore, '#6c757d')
		FROM apparati_nave a
		LEFT JOIN tipi_apparato t ON a.tipo = t.nome
		WHERE a.nave_id = ?
		ORDER BY a.tipo, a.nome
	`, naveID)
	if err != nil {
		return apparati
	}
	defer rows.Close()

	for rows.Next() {
		var a models.ApparatoNave
		var httpsEnabled int
		var ultimoCheck sql.NullString

		rows.Scan(
			&a.ID, &a.NaveID, &a.Nome, &a.Tipo, &a.IP, &a.MAC, &a.Vendor, &a.Modello,
			&a.Firmware, &a.Location, &a.SNMPCommunity, &a.SSHUser, &a.SSHPass, &a.SSHPort,
			&a.HTTPUser, &a.HTTPPass, &a.HTTPPort, &httpsEnabled, &a.Note,
			&ultimoCheck, &a.Stato, &a.TipoIcona, &a.TipoColore,
		)
		a.HTTPSEnabled = httpsEnabled == 1
		if ultimoCheck.Valid { a.UltimoCheck = ultimoCheck.String }
		apparati = append(apparati, a)
	}
	return apparati
}

func getApparatoByID(id int64) models.ApparatoNave {
	var a models.ApparatoNave
	var httpsEnabled int
	var ultimoCheck sql.NullString

	database.DB.QueryRow(`
		SELECT id, nave_id, nome, tipo, ip, mac, vendor, modello, firmware, location,
		       snmp_community, ssh_user, ssh_pass, ssh_port, http_user, http_pass,
		       http_port, https_enabled, note, ultimo_check, stato
		FROM apparati_nave WHERE id = ?
	`, id).Scan(
		&a.ID, &a.NaveID, &a.Nome, &a.Tipo, &a.IP, &a.MAC, &a.Vendor, &a.Modello,
		&a.Firmware, &a.Location, &a.SNMPCommunity, &a.SSHUser, &a.SSHPass, &a.SSHPort,
		&a.HTTPUser, &a.HTTPPass, &a.HTTPPort, &httpsEnabled, &a.Note,
		&ultimoCheck, &a.Stato,
	)
	a.HTTPSEnabled = httpsEnabled == 1
	if ultimoCheck.Valid { a.UltimoCheck = ultimoCheck.String }
	return a
}

func getTipiApparato() []models.TipoApparato {
	var tipi []models.TipoApparato
	rows, _ := database.DB.Query("SELECT id, nome, icona, colore FROM tipi_apparato ORDER BY nome")
	defer rows.Close()
	for rows.Next() {
		var t models.TipoApparato
		rows.Scan(&t.ID, &t.Nome, &t.Icona, &t.Colore)
		tipi = append(tipi, t)
	}
	return tipi
}

func discoverDevicesFromObservium(nave models.Nave) ([]models.DeviceDiscovery, error) {
	var devices []models.DeviceDiscovery

	// Connessione SSH a Observium per eseguire query al database
	if nave.ObserviumSSHUser == "" || nave.ObserviumSSHPass == "" {
		return devices, fmt.Errorf("credenziali SSH Observium non configurate")
	}

	sshPort := nave.ObserviumSSHPort
	if sshPort == 0 { sshPort = 22 }

	// Usa sshpass per connettersi e ottenere lista dispositivi
	cmd := exec.Command("sshpass", "-p", nave.ObserviumSSHPass, "ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		"-p", fmt.Sprintf("%d", sshPort),
		fmt.Sprintf("%s@%s", nave.ObserviumSSHUser, nave.ObserviumIP),
		"grep db_pass /opt/observium/config.php 2>/dev/null | cut -d' -f2 | xargs -I{} mysql -u observium -p{} observium -N -e \"SELECT hostname, ip, hardware, os, location FROM devices WHERE disabled = 0\"")

	output, err := cmd.Output()
	if err != nil {
		// Fallback: prova con snmpwalk se disponibile
		return discoverViaSNMP(nave)
	}

	// Parse output MySQL
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" { continue }
		fields := strings.Split(line, "\t")
		if len(fields) >= 2 {
			device := models.DeviceDiscovery{
				Hostname: fields[0],
				IP:       fields[1],
			}
			if len(fields) > 2 { device.Vendor = fields[2] }
			if len(fields) > 3 { device.Type = fields[3] }
			if len(fields) > 4 { device.Location = fields[4] }
			device.Status = "discovered"
			devices = append(devices, device)
		}
	}

	return devices, nil
}

func discoverViaSNMP(nave models.Nave) ([]models.DeviceDiscovery, error) {
	var devices []models.DeviceDiscovery

	community := nave.SNMPCommunity
	if community == "" { community = "public" }

	// Esegui snmpwalk per discovery base
	cmd := exec.Command("snmpwalk", "-v2c", "-c", community, nave.ObserviumIP, "1.3.6.1.2.1.1.5.0")
	output, err := cmd.Output()
	if err != nil {
		return devices, fmt.Errorf("SNMP non raggiungibile")
	}

	// Almeno Observium è raggiungibile
	hostname := strings.TrimSpace(string(output))
	device := models.DeviceDiscovery{
		Hostname: hostname,
		IP:       nave.ObserviumIP,
		Type:     "Server",
		Status:   "discovered",
	}
	devices = append(devices, device)

	return devices, nil
}
