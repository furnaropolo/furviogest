package main

import (
	"database/sql"
	"log"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "/home/ies/furviogest/data/furviogest.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Aggiungi campi Observium alla tabella navi
	queries := []string{
		`ALTER TABLE navi ADD COLUMN observium_ip TEXT DEFAULT ''`,
		`ALTER TABLE navi ADD COLUMN observium_user TEXT DEFAULT ''`,
		`ALTER TABLE navi ADD COLUMN observium_pass TEXT DEFAULT ''`,
		`ALTER TABLE navi ADD COLUMN observium_ssh_user TEXT DEFAULT ''`,
		`ALTER TABLE navi ADD COLUMN observium_ssh_pass TEXT DEFAULT ''`,
		`ALTER TABLE navi ADD COLUMN observium_ssh_port INTEGER DEFAULT 22`,
		`ALTER TABLE navi ADD COLUMN snmp_community TEXT DEFAULT 'public'`,
	}

	for _, q := range queries {
		db.Exec(q) // Ignora errori se colonna esiste già
	}

	// Tabella apparati nave
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS apparati_nave (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nave_id INTEGER NOT NULL,
			nome TEXT NOT NULL,
			tipo TEXT DEFAULT '',
			ip TEXT DEFAULT '',
			mac TEXT DEFAULT '',
			vendor TEXT DEFAULT '',
			modello TEXT DEFAULT '',
			firmware TEXT DEFAULT '',
			location TEXT DEFAULT '',
			snmp_community TEXT DEFAULT '',
			ssh_user TEXT DEFAULT '',
			ssh_pass TEXT DEFAULT '',
			ssh_port INTEGER DEFAULT 22,
			http_user TEXT DEFAULT '',
			http_pass TEXT DEFAULT '',
			http_port INTEGER DEFAULT 80,
			https_enabled INTEGER DEFAULT 0,
			note TEXT DEFAULT '',
			ultimo_check TEXT,
			stato TEXT DEFAULT 'unknown',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (nave_id) REFERENCES navi(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		log.Fatal("Errore creazione tabella apparati_nave:", err)
	}

	// Tabella tipi apparato predefiniti
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tipi_apparato (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nome TEXT NOT NULL UNIQUE,
			icona TEXT DEFAULT 'device',
			colore TEXT DEFAULT '#6c757d'
		)
	`)
	if err != nil {
		log.Fatal("Errore creazione tabella tipi_apparato:", err)
	}

	// Inserisci tipi predefiniti
	tipi := []struct{ nome, icona, colore string }{
		{"Router", "router", "#007bff"},
		{"Switch", "switch", "#28a745"},
		{"Access Point", "wifi", "#17a2b8"},
		{"Firewall", "shield", "#dc3545"},
		{"Server", "server", "#6f42c1"},
		{"VM", "cloud", "#fd7e14"},
		{"Antenna VSAT", "satellite", "#20c997"},
		{"Antenna Starlink", "satellite-dish", "#e83e8c"},
		{"Modem", "modem", "#6c757d"},
		{"UPS", "battery", "#ffc107"},
		{"IP-PDU", "plug", "#795548"},
		{"NAS", "hdd", "#607d8b"},
		{"Telecamera", "camera", "#9c27b0"},
		{"DAS GSM", "signal", "#ff5722"},
		{"Remote Unit", "broadcast", "#00bcd4"},
		{"Controller WiFi", "wifi", "#4caf50"},
		{"Altro", "device", "#9e9e9e"},
	}

	for _, t := range tipi {
		db.Exec("INSERT OR IGNORE INTO tipi_apparato (nome, icona, colore) VALUES (?, ?, ?)", t.nome, t.icona, t.colore)
	}

	log.Println("Tabelle apparati create con successo!")
}
