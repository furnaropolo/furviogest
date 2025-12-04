package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

// InitDB inizializza la connessione al database
func InitDB(dbPath string) error {
	// Crea la directory se non esiste
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("errore creazione directory database: %w", err)
	}

	var err error
	DB, err = sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return fmt.Errorf("errore apertura database: %w", err)
	}

	// Test connessione
	if err = DB.Ping(); err != nil {
		return fmt.Errorf("errore connessione database: %w", err)
	}

	// Crea le tabelle
	if err = createTables(); err != nil {
		return fmt.Errorf("errore creazione tabelle: %w", err)
	}

	log.Println("Database inizializzato correttamente")
	return nil
}

// CloseDB chiude la connessione al database
func CloseDB() {
	if DB != nil {
		DB.Close()
	}
}

func createTables() error {
	schema := `
	-- Tabella utenti (tecnici e guest)
	CREATE TABLE IF NOT EXISTS utenti (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		nome TEXT NOT NULL,
		cognome TEXT NOT NULL,
		email TEXT NOT NULL,
		telefono TEXT,
		ruolo TEXT NOT NULL DEFAULT 'guest' CHECK(ruolo IN ('tecnico', 'guest')),
		attivo INTEGER NOT NULL DEFAULT 1,
		documento_path TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Tabella fornitori
	CREATE TABLE IF NOT EXISTS fornitori (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nome TEXT NOT NULL,
		indirizzo TEXT,
		telefono TEXT,
		email TEXT,
		note TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Tabella porti
	CREATE TABLE IF NOT EXISTS porti (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nome TEXT NOT NULL,
		citta TEXT,
		paese TEXT,
		nome_agenzia TEXT,
		email_agenzia TEXT,
		telefono_agenzia TEXT,
		note TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Tabella automezzi
	CREATE TABLE IF NOT EXISTS automezzi (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		targa TEXT UNIQUE NOT NULL,
		marca TEXT,
		modello TEXT,
		libretto_path TEXT,
		note TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Tabella compagnie di navigazione
	CREATE TABLE IF NOT EXISTS compagnie (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nome TEXT NOT NULL,
		indirizzo TEXT,
		telefono TEXT,
		email TEXT,
		note TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Tabella navi
	CREATE TABLE IF NOT EXISTS navi (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		compagnia_id INTEGER NOT NULL,
		nome TEXT NOT NULL,
		imo TEXT,
		email_master TEXT,
		email_direttore_macchina TEXT,
		email_ispettore TEXT,
		note TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (compagnia_id) REFERENCES compagnie(id) ON DELETE CASCADE
	);

	-- Tabella prodotti (magazzino)
	CREATE TABLE IF NOT EXISTS prodotti (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		codice TEXT UNIQUE NOT NULL,
		nome TEXT NOT NULL,
		descrizione TEXT,
		categoria TEXT NOT NULL DEFAULT 'materiale' CHECK(categoria IN ('materiale', 'cavo')),
		tipo TEXT NOT NULL CHECK(tipo IN ('wifi', 'gsm', 'entrambi')),
		origine TEXT NOT NULL CHECK(origine IN ('spare', 'nuovo')),
		fornitore_id INTEGER,
		numero_fattura TEXT,
		data_fattura DATE,
		nave_origine TEXT,
		giacenza REAL NOT NULL DEFAULT 0,
		giacenza_minima REAL NOT NULL DEFAULT 0,
		unita_misura TEXT NOT NULL DEFAULT 'pz' CHECK(unita_misura IN ('pz', 'm')),
		note TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (fornitore_id) REFERENCES fornitori(id) ON DELETE SET NULL
	);

	-- Tabella movimenti magazzino
	CREATE TABLE IF NOT EXISTS movimenti_magazzino (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		prodotto_id INTEGER NOT NULL,
		tecnico_id INTEGER NOT NULL,
		quantita REAL NOT NULL,
		tipo TEXT NOT NULL CHECK(tipo IN ('carico', 'scarico')),
		motivo TEXT,
		rapporto_id INTEGER,
		ddt_id INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (prodotto_id) REFERENCES prodotti(id) ON DELETE CASCADE,
		FOREIGN KEY (tecnico_id) REFERENCES utenti(id) ON DELETE CASCADE,
		FOREIGN KEY (rapporto_id) REFERENCES rapporti_intervento(id) ON DELETE SET NULL,
		FOREIGN KEY (ddt_id) REFERENCES ddt(id) ON DELETE SET NULL
	);

	-- Tabella richieste permesso
	CREATE TABLE IF NOT EXISTS richieste_permesso (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nave_id INTEGER NOT NULL,
		porto_id INTEGER NOT NULL,
		tecnico_creatore INTEGER NOT NULL,
		automezzo_id INTEGER,
		targa_esterna TEXT,
		tipo_durata TEXT NOT NULL CHECK(tipo_durata IN ('giornaliera', 'multigiorno', 'fine_lavori')),
		data_inizio DATE NOT NULL,
		data_fine DATE,
		note TEXT,
		email_inviata INTEGER NOT NULL DEFAULT 0,
		data_invio_email DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (nave_id) REFERENCES navi(id) ON DELETE CASCADE,
		FOREIGN KEY (porto_id) REFERENCES porti(id) ON DELETE CASCADE,
		FOREIGN KEY (tecnico_creatore) REFERENCES utenti(id) ON DELETE CASCADE,
		FOREIGN KEY (automezzo_id) REFERENCES automezzi(id) ON DELETE SET NULL
	);

	-- Tabella tecnici associati a richiesta permesso
	CREATE TABLE IF NOT EXISTS tecnici_permesso (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		richiesta_permesso_id INTEGER NOT NULL,
		tecnico_id INTEGER NOT NULL,
		FOREIGN KEY (richiesta_permesso_id) REFERENCES richieste_permesso(id) ON DELETE CASCADE,
		FOREIGN KEY (tecnico_id) REFERENCES utenti(id) ON DELETE CASCADE,
		UNIQUE(richiesta_permesso_id, tecnico_id)
	);

	-- Tabella rapporti intervento
	CREATE TABLE IF NOT EXISTS rapporti_intervento (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nave_id INTEGER NOT NULL,
		porto_id INTEGER NOT NULL,
		tipo TEXT NOT NULL CHECK(tipo IN ('wifi', 'gsm')),
		data_intervento DATE NOT NULL,
		descrizione TEXT,
		note TEXT,
		ddt_generato INTEGER NOT NULL DEFAULT 0,
		numero_ddt TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (nave_id) REFERENCES navi(id) ON DELETE CASCADE,
		FOREIGN KEY (porto_id) REFERENCES porti(id) ON DELETE CASCADE
	);

	-- Tabella tecnici associati a rapporto intervento
	CREATE TABLE IF NOT EXISTS tecnici_rapporto (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		rapporto_id INTEGER NOT NULL,
		tecnico_id INTEGER NOT NULL,
		FOREIGN KEY (rapporto_id) REFERENCES rapporti_intervento(id) ON DELETE CASCADE,
		FOREIGN KEY (tecnico_id) REFERENCES utenti(id) ON DELETE CASCADE,
		UNIQUE(rapporto_id, tecnico_id)
	);

	-- Tabella foto rapporto
	CREATE TABLE IF NOT EXISTS foto_rapporto (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		rapporto_id INTEGER NOT NULL,
		file_path TEXT NOT NULL,
		descrizione TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (rapporto_id) REFERENCES rapporti_intervento(id) ON DELETE CASCADE
	);

	-- Tabella materiale utilizzato nel rapporto
	CREATE TABLE IF NOT EXISTS materiale_rapporto (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		rapporto_id INTEGER NOT NULL,
		prodotto_id INTEGER NOT NULL,
		quantita INTEGER NOT NULL,
		FOREIGN KEY (rapporto_id) REFERENCES rapporti_intervento(id) ON DELETE CASCADE,
		FOREIGN KEY (prodotto_id) REFERENCES prodotti(id) ON DELETE CASCADE
	);

	-- Tabella trasferte
	CREATE TABLE IF NOT EXISTS trasferte (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tecnico_id INTEGER NOT NULL,
		rapporto_id INTEGER,
		destinazione TEXT NOT NULL,
		data_partenza DATE NOT NULL,
		data_rientro DATE NOT NULL,
		pernottamento INTEGER NOT NULL DEFAULT 0,
		numero_notti INTEGER NOT NULL DEFAULT 0,
		note TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (tecnico_id) REFERENCES utenti(id) ON DELETE CASCADE,
		FOREIGN KEY (rapporto_id) REFERENCES rapporti_intervento(id) ON DELETE SET NULL
	);

	-- Tabella note spese
	CREATE TABLE IF NOT EXISTS note_spese (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tecnico_id INTEGER NOT NULL,
		trasferta_id INTEGER,
		data DATE NOT NULL,
		tipo_spesa TEXT NOT NULL CHECK(tipo_spesa IN ('carburante', 'hotel', 'pranzo', 'cena', 'materiali', 'varie')),
		descrizione TEXT NOT NULL,
		importo REAL NOT NULL,
		metodo_pagamento TEXT NOT NULL CHECK(metodo_pagamento IN ('carta_aziendale', 'tecnico')),
		ricevuta_path TEXT,
		note TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (tecnico_id) REFERENCES utenti(id) ON DELETE CASCADE,
		FOREIGN KEY (trasferta_id) REFERENCES trasferte(id) ON DELETE SET NULL
	);

	-- Tabella DDT
	CREATE TABLE IF NOT EXISTS ddt (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		numero TEXT UNIQUE NOT NULL,
		tipo_ddt TEXT NOT NULL DEFAULT 'intervento' CHECK(tipo_ddt IN ('intervento', 'spedizione')),
		rapporto_id INTEGER,
		nave_id INTEGER NOT NULL,
		compagnia_id INTEGER NOT NULL,
		porto_id INTEGER,
		destinatario TEXT,
		indirizzo TEXT,
		vettore TEXT,
		data_emissione DATE NOT NULL,
		note TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (rapporto_id) REFERENCES rapporti_intervento(id) ON DELETE SET NULL,
		FOREIGN KEY (nave_id) REFERENCES navi(id) ON DELETE CASCADE,
		FOREIGN KEY (compagnia_id) REFERENCES compagnie(id) ON DELETE CASCADE,
		FOREIGN KEY (porto_id) REFERENCES porti(id) ON DELETE SET NULL
	);

	-- Tabella righe DDT
	CREATE TABLE IF NOT EXISTS righe_ddt (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ddt_id INTEGER NOT NULL,
		prodotto_id INTEGER NOT NULL,
		quantita REAL NOT NULL,
		descrizione TEXT,
		FOREIGN KEY (ddt_id) REFERENCES ddt(id) ON DELETE CASCADE,
		FOREIGN KEY (prodotto_id) REFERENCES prodotti(id) ON DELETE CASCADE
	);

	-- Tabella impostazioni azienda (record singolo)
	CREATE TABLE IF NOT EXISTS impostazioni_azienda (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		ragione_sociale TEXT NOT NULL DEFAULT '',
		partita_iva TEXT DEFAULT '',
		codice_fiscale TEXT DEFAULT '',
		indirizzo TEXT DEFAULT '',
		cap TEXT DEFAULT '',
		citta TEXT DEFAULT '',
		provincia TEXT DEFAULT '',
		telefono TEXT DEFAULT '',
		email TEXT DEFAULT '',
		pec TEXT DEFAULT '',
		sito_web TEXT DEFAULT '',
		logo_path TEXT DEFAULT '',
		firma_email_path TEXT DEFAULT '',
		firma_email_testo TEXT DEFAULT '',
		iban TEXT DEFAULT '',
		banca TEXT DEFAULT '',
		codice_sdi TEXT DEFAULT '',
		note TEXT DEFAULT '',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Inserisce record iniziale impostazioni azienda se non esiste
	INSERT OR IGNORE INTO impostazioni_azienda (id) VALUES (1);

	-- Indici per migliorare le performance
	CREATE INDEX IF NOT EXISTS idx_navi_compagnia ON navi(compagnia_id);
	CREATE INDEX IF NOT EXISTS idx_prodotti_tipo ON prodotti(tipo);
	CREATE INDEX IF NOT EXISTS idx_prodotti_origine ON prodotti(origine);
	CREATE INDEX IF NOT EXISTS idx_movimenti_prodotto ON movimenti_magazzino(prodotto_id);
	CREATE INDEX IF NOT EXISTS idx_movimenti_tecnico ON movimenti_magazzino(tecnico_id);
	CREATE INDEX IF NOT EXISTS idx_richieste_nave ON richieste_permesso(nave_id);
	CREATE INDEX IF NOT EXISTS idx_richieste_porto ON richieste_permesso(porto_id);
	CREATE INDEX IF NOT EXISTS idx_rapporti_nave ON rapporti_intervento(nave_id);
	CREATE INDEX IF NOT EXISTS idx_rapporti_data ON rapporti_intervento(data_intervento);
	CREATE INDEX IF NOT EXISTS idx_trasferte_tecnico ON trasferte(tecnico_id);
	CREATE INDEX IF NOT EXISTS idx_trasferte_data ON trasferte(data_partenza);
	CREATE INDEX IF NOT EXISTS idx_note_spese_tecnico ON note_spese(tecnico_id);
	CREATE INDEX IF NOT EXISTS idx_note_spese_data ON note_spese(data);
	CREATE INDEX IF NOT EXISTS idx_prodotti_categoria ON prodotti(categoria);
	CREATE INDEX IF NOT EXISTS idx_ddt_tipo ON ddt(tipo_ddt);
	CREATE INDEX IF NOT EXISTS idx_ddt_nave ON ddt(nave_id);
	CREATE INDEX IF NOT EXISTS idx_righe_ddt_ddt ON righe_ddt(ddt_id);
	`

	_, err := DB.Exec(schema)
	return err
}

// CreateDefaultAdmin crea l'utente admin predefinito se non esiste
func CreateDefaultAdmin(hashPassword func(string) (string, error)) error {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM utenti WHERE username = 'admin'").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Password default: admin (da cambiare al primo accesso)
		hashedPassword, err := hashPassword("admin")
		if err != nil {
			return err
		}
		_, err = DB.Exec(`
			INSERT INTO utenti (username, password, nome, cognome, email, ruolo, attivo)
			VALUES ('admin', ?, 'Admin', 'Sistema', 'admin@furviogest.local', 'tecnico', 1)
		`, hashedPassword)
		if err != nil {
			return err
		}
		log.Println("Utente admin predefinito creato (username: admin, password: admin)")
	}

	return nil
}
