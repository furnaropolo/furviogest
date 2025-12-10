package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"furviogest/internal/database"
)

// ============================================
// STRUTTURE DATI
// ============================================

type SalaServer struct {
	ID        int64
	Nome      string
	Indirizzo string
	Citta     string
	CAP       string
	Telefono  string
	Email     string
	Note      string
}

type SwitchSalaServer struct {
	ID           int64
	SalaServerID int64
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
	PorteTotali  int
	PorteLibere  int
	UltimoCheck  string
}

type SalaServerPageData struct {
	SalaServer SalaServer
	Switches   []SwitchSalaServer
	Backups    []ConfigBackup
}

// ============================================
// HANDLERS LISTA SALE SERVER
// ============================================

func ListaSaleServer(w http.ResponseWriter, r *http.Request) {
	data := NewPageData("Sale Server - FurvioGest", r)

	rows, err := database.DB.Query("SELECT id, nome, COALESCE(indirizzo, ''), COALESCE(citta, ''), COALESCE(telefono, ''), COALESCE(email, '') FROM sale_server ORDER BY nome")
	if err != nil {
		http.Error(w, "Errore caricamento sale server", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var saleServer []SalaServer
	for rows.Next() {
		var s SalaServer
		rows.Scan(&s.ID, &s.Nome, &s.Indirizzo, &s.Citta, &s.Telefono, &s.Email)
		saleServer = append(saleServer, s)
	}

	data.Data = saleServer
	renderTemplate(w, "sale_server.html", data)
}

func NuovaSalaServer(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		data := NewPageData("Nuova Sala Server - FurvioGest", r)
		data.Data = SalaServer{}
		renderTemplate(w, "sala_server_form.html", data)
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

	_, err := database.DB.Exec("INSERT INTO sale_server (nome, indirizzo, citta, cap, telefono, email, note) VALUES (?, ?, ?, ?, ?, ?, ?)",
		nome, indirizzo, citta, cap, telefono, email, note)
	if err != nil {
		http.Error(w, "Errore salvataggio: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/sale-server", http.StatusSeeOther)
}

func ModificaSalaServer(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/sale-server/modifica/")
	id, _ := strconv.ParseInt(path, 10, 64)

	if r.Method == http.MethodGet {
		var s SalaServer
		err := database.DB.QueryRow("SELECT id, nome, COALESCE(indirizzo, ''), COALESCE(citta, ''), COALESCE(cap, ''), COALESCE(telefono, ''), COALESCE(email, ''), COALESCE(note, '') FROM sale_server WHERE id = ?", id).
			Scan(&s.ID, &s.Nome, &s.Indirizzo, &s.Citta, &s.CAP, &s.Telefono, &s.Email, &s.Note)
		if err != nil {
			http.Redirect(w, r, "/sale-server", http.StatusSeeOther)
			return
		}
		data := NewPageData("Modifica Sala Server - FurvioGest", r)
		data.Data = s
		renderTemplate(w, "sala_server_form.html", data)
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

	database.DB.Exec("UPDATE sale_server SET nome = ?, indirizzo = ?, citta = ?, cap = ?, telefono = ?, email = ?, note = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		nome, indirizzo, citta, cap, telefono, email, note, id)

	http.Redirect(w, r, "/sale-server", http.StatusSeeOther)
}

func EliminaSalaServer(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/sale-server/elimina/")
	id, _ := strconv.ParseInt(path, 10, 64)

	database.DB.Exec("DELETE FROM sale_server WHERE id = ?", id)
	http.Redirect(w, r, "/sale-server", http.StatusSeeOther)
}

// ============================================
// GESTIONE RETE SALA SERVER
// ============================================

func GestioneReteSalaServer(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/sale-server/rete/")
	salaServerID, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/sale-server", http.StatusSeeOther)
		return
	}

	// Carica sala server
	var salaServer SalaServer
	err = database.DB.QueryRow("SELECT id, nome, COALESCE(indirizzo, ''), COALESCE(citta, '') FROM sale_server WHERE id = ?", salaServerID).
		Scan(&salaServer.ID, &salaServer.Nome, &salaServer.Indirizzo, &salaServer.Citta)
	if err != nil {
		http.Redirect(w, r, "/sale-server", http.StatusSeeOther)
		return
	}

	// Carica switch
	switches := getSwitchesSalaServer(salaServerID)

	// Carica backup recenti
	backups := getBackupsSalaServer(salaServerID, 10)

	pageData := SalaServerPageData{
		SalaServer: salaServer,
		Switches:   switches,
		Backups:    backups,
	}

	data := NewPageData("Gestione Rete - "+salaServer.Nome, r)
	data.Data = pageData
	renderTemplate(w, "rete_sala_server.html", data)
}

// ============================================
// SWITCH SALA SERVER
// ============================================

func NuovoSwitchSalaServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metodo non consentito", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/sale-server/switch/nuovo/")
	salaServerID, _ := strconv.ParseInt(path, 10, 64)

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
	log.Printf("[NUOVO SWITCH SALA SERVER] Hostname recuperato: %s", nome)

	_, err := database.DB.Exec("INSERT INTO switch_sala_server (sala_server_id, nome, marca, modello, ip, ssh_port, ssh_user, ssh_pass, note, protocollo) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		salaServerID, nome, marca, modello, ip, sshPort, sshUser, sshPass, note, protocollo)

	if err != nil {
		http.Error(w, "Errore salvataggio: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/sale-server/rete/%d", salaServerID), http.StatusSeeOther)
}

func ModificaSwitchSalaServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metodo non consentito", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/sale-server/switch/modifica/")
	switchID, _ := strconv.ParseInt(path, 10, 64)

	var salaServerID int64
	database.DB.QueryRow("SELECT sala_server_id FROM switch_sala_server WHERE id = ?", switchID).Scan(&salaServerID)

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

	database.DB.Exec("UPDATE switch_sala_server SET marca = ?, modello = ?, ip = ?, ssh_port = ?, ssh_user = ?, ssh_pass = ?, note = ?, protocollo = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		marca, modello, ip, sshPort, sshUser, sshPass, note, protocollo, switchID)

	http.Redirect(w, r, fmt.Sprintf("/sale-server/rete/%d", salaServerID), http.StatusSeeOther)
}

func EliminaSwitchSalaServer(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/sale-server/switch/elimina/")
	switchID, _ := strconv.ParseInt(path, 10, 64)

	var salaServerID int64
	database.DB.QueryRow("SELECT sala_server_id FROM switch_sala_server WHERE id = ?", switchID).Scan(&salaServerID)

	// Elimina file backup fisici e record dal database (tipo_apparato può essere 'switch' o 'switch_sala_server')
	rows, _ := database.DB.Query("SELECT file_path FROM config_backup_ufficio WHERE (tipo_apparato = 'switch' OR tipo_apparato = 'switch_sala_server') AND apparato_id = ? AND sala_server_id IS NOT NULL", switchID)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var filePath string
			rows.Scan(&filePath)
			if filePath != "" {
				os.Remove(filePath)
			}
		}
	}
	database.DB.Exec("DELETE FROM config_backup_ufficio WHERE (tipo_apparato = 'switch' OR tipo_apparato = 'switch_sala_server') AND apparato_id = ? AND sala_server_id IS NOT NULL", switchID)

	database.DB.Exec("DELETE FROM switch_sala_server WHERE id = ?", switchID)
	http.Redirect(w, r, fmt.Sprintf("/sale-server/rete/%d", salaServerID), http.StatusSeeOther)
}

// ============================================
// FUNZIONI HELPER
// ============================================

func getSwitchesSalaServer(salaServerID int64) []SwitchSalaServer {
	var switches []SwitchSalaServer
	rows, err := database.DB.Query("SELECT id, sala_server_id, nome, marca, COALESCE(modello, ''), ip, ssh_port, ssh_user, ssh_pass, COALESCE(protocollo, 'ssh'), COALESCE(note, ''), ultimo_backup, COALESCE(porte_totali, 0), COALESCE(porte_libere, 0), ultimo_check FROM switch_sala_server WHERE sala_server_id = ? ORDER BY nome", salaServerID)
	if err != nil {
		return switches
	}
	defer rows.Close()

	for rows.Next() {
		var sw SwitchSalaServer
		var ultimoBackup, ultimoCheck sql.NullString
		rows.Scan(&sw.ID, &sw.SalaServerID, &sw.Nome, &sw.Marca, &sw.Modello, &sw.IP, &sw.SSHPort, &sw.SSHUser, &sw.SSHPass, &sw.Protocollo, &sw.Note, &ultimoBackup, &sw.PorteTotali, &sw.PorteLibere, &ultimoCheck)
		if ultimoBackup.Valid {
			sw.UltimoBackup = ultimoBackup.String
		}
		if ultimoCheck.Valid {
			sw.UltimoCheck = ultimoCheck.String
		}
		switches = append(switches, sw)
	}
	return switches
}

func getBackupsSalaServer(salaServerID int64, limit int) []ConfigBackup {
	var backups []ConfigBackup
	rows, err := database.DB.Query("SELECT id, COALESCE(sala_server_id, 0), tipo_apparato, apparato_id, nome_apparato, file_path, file_size, hash_md5, created_at FROM config_backup_ufficio WHERE sala_server_id = ? ORDER BY created_at DESC LIMIT ?", salaServerID, limit)
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
