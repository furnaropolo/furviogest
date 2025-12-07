package models

import (
	"time"
)

// Ruolo utente
type Ruolo string

const (
	RuoloTecnico Ruolo = "tecnico"
	RuoloGuest         Ruolo = "guest"
	RuoloAmministrazione Ruolo = "amministrazione"
)

// Utente rappresenta un utente del sistema
type Utente struct {
	ID            int64     `json:"id"`
	Username      string    `json:"username"`
	Password      string    `json:"-"` // Non esporre la password in JSON
	Nome          string    `json:"nome"`
	Cognome       string    `json:"cognome"`
	Email         string    `json:"email"`
	Telefono      string    `json:"telefono"`
	Ruolo         Ruolo     `json:"ruolo"`
	Attivo        bool      `json:"attivo"`
	DocumentoPath string    `json:"documento_path"` // Path al documento di identità
	// Impostazioni SMTP personali
	SMTPServer    string    `json:"smtp_server"`
	SMTPPort      int       `json:"smtp_port"`
	SMTPUser      string    `json:"smtp_user"`
	SMTPPassword  string    `json:"-"` // Non esporre in JSON
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Fornitore rappresenta un fornitore
type Fornitore struct {
	ID        int64     `json:"id"`
	Nome      string    `json:"nome"`
	Indirizzo string    `json:"indirizzo"`
	Telefono  string    `json:"telefono"`
	Email     string    `json:"email"`
	Note              string `json:"note"`
	EmailDestinatari  string `json:"email_destinatari"` // solo_agenzia o tutti`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Porto rappresenta un porto
type Porto struct {
	ID            int64     `json:"id"`
	Nome          string    `json:"nome"`
	Citta         string    `json:"citta"`
	Paese         string    `json:"paese"`
	NomeAgenzia   string    `json:"nome_agenzia"`
	EmailAgenzia  string    `json:"email_agenzia"`
	TelefonoAgenzia string  `json:"telefono_agenzia"`
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Automezzo rappresenta un veicolo aziendale
type Automezzo struct {
	ID           int64     `json:"id"`
	Targa        string    `json:"targa"`
	Marca        string    `json:"marca"`
	Modello      string    `json:"modello"`
	LibrettoPath string    `json:"libretto_path"` // Path al libretto di circolazione
	Note         string    `json:"note"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Compagnia rappresenta una compagnia di navigazione
type Compagnia struct {
	ID        int64     `json:"id"`
	Nome      string    `json:"nome"`
	Indirizzo string    `json:"indirizzo"`
	Telefono  string    `json:"telefono"`
	Email     string    `json:"email"`
	Note              string `json:"note"`
	EmailDestinatari  string `json:"email_destinatari"` // solo_agenzia o tutti`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Nave rappresenta una nave di una compagnia
type Nave struct {
	ID                   int64     `json:"id"`
	CompagniaID          int64     `json:"compagnia_id"`
	Nome                 string    `json:"nome"`
	IMO                  string    `json:"imo"` // Numero IMO della nave
	EmailMaster          string    `json:"email_master"`
	EmailDirettoreMacchina string  `json:"email_direttore_macchina"`
	EmailIspettore       string    `json:"email_ispettore"`
	Note                 string    `json:"note"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	// Stato lavori
	FermaPerLavori       bool       `json:"ferma_per_lavori"`
	DataInizioLavori     *time.Time `json:"data_inizio_lavori,omitempty"`
	DataFineLavoriPrev   *time.Time `json:"data_fine_lavori_prevista,omitempty"`
	PortoLavoriID        *int64     `json:"porto_lavori_id,omitempty"`
	// Campo virtuale per porto lavori
	NomePortoLavori      string     `json:"nome_porto_lavori,omitempty"`
	// Campo virtuale per join
	NomeCompagnia        string    `json:"nome_compagnia,omitempty"`
	// Observium config
	ObserviumIP          string    `json:"observium_ip,omitempty"`
	ObserviumUser        string    `json:"observium_user,omitempty"`
	ObserviumPass        string    `json:"observium_pass,omitempty"`
	ObserviumSSHUser     string    `json:"observium_ssh_user,omitempty"`
	ObserviumSSHPass     string    `json:"observium_ssh_pass,omitempty"`
	ObserviumSSHPort     int       `json:"observium_ssh_port,omitempty"`
	SNMPCommunity        string    `json:"snmp_community,omitempty"`
}

// TipoProdotto indica se il prodotto è per WiFi, GSM o entrambi
type TipoProdotto string

const (
	TipoWiFi     TipoProdotto = "wifi"
	TipoGSM      TipoProdotto = "gsm"
	TipoEntrambi TipoProdotto = "entrambi"
)

// CategoriaProdotto indica la categoria del prodotto
type CategoriaProdotto string

const (
	CategoriaMateriale CategoriaProdotto = "materiale" // Prodotti normali (pezzi)
	CategoriaCavo      CategoriaProdotto = "cavo"      // Cavi (metri)
)

// OrigineProdotto indica se il prodotto è spare o nuovo
type OrigineProdotto string

const (
	OrigineSpare  OrigineProdotto = "spare"
	OrigineNuovo  OrigineProdotto = "nuovo"
)

// Prodotto rappresenta un prodotto in magazzino
type Prodotto struct {
	ID              int64             `json:"id"`
	Codice          string            `json:"codice"`
	Nome            string            `json:"nome"`
	Descrizione     string            `json:"descrizione"`
	Categoria       CategoriaProdotto `json:"categoria"` // materiale o cavo
	Tipo            TipoProdotto      `json:"tipo"`      // wifi, gsm o entrambi
	Origine         OrigineProdotto   `json:"origine"`   // spare o nuovo
	FornitoreID     *int64            `json:"fornitore_id,omitempty"`
	NumeroFattura   string            `json:"numero_fattura,omitempty"`
	DataFattura     *time.Time        `json:"data_fattura,omitempty"`
	NaveOrigine     string            `json:"nave_origine,omitempty"`
	Giacenza        float64           `json:"giacenza"`         // float per supportare metri decimali
	GiacenzaMinima  float64           `json:"giacenza_minima"`
	UnitaMisura     string            `json:"unita_misura"`     // "pz" o "m" (metri)
	Note            string            `json:"note"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	// Campi virtuali per join
	NomeFornitore   string            `json:"nome_fornitore,omitempty"`
}

// MovimentoMagazzino rappresenta un movimento di carico/scarico
type MovimentoMagazzino struct {
	ID          int64     `json:"id"`
	ProdottoID  int64     `json:"prodotto_id"`
	TecnicoID   int64     `json:"tecnico_id"`
	Quantita    float64   `json:"quantita"` // float per supportare metri decimali
	Tipo        string    `json:"tipo"`     // "carico" o "scarico"
	Motivo      string    `json:"motivo"`   // es. "Intervento su nave X", "Acquisto", ecc.
	RapportoID  *int64    `json:"rapporto_id,omitempty"`
	DDTID       *int64    `json:"ddt_id,omitempty"` // Collegamento al DDT se spedizione
	CreatedAt   time.Time `json:"created_at"`
	// Campi virtuali
	NomeProdotto  string  `json:"nome_prodotto,omitempty"`
	NomeTecnico   string  `json:"nome_tecnico,omitempty"`
	UnitaMisura   string  `json:"unita_misura,omitempty"`
}

// TipoDurataPermesso indica la durata del permesso
type TipoDurataPermesso string

const (
	DurataGiornaliera   TipoDurataPermesso = "giornaliera"
	DurataMultigiorno   TipoDurataPermesso = "multigiorno"
	DurataFineLavori    TipoDurataPermesso = "fine_lavori"
)

// RichiestaPermesso rappresenta una richiesta di accesso al porto
type RichiestaPermesso struct {
	ID              int64              `json:"id"`
	NaveID          int64              `json:"nave_id"`
	PortoID         int64              `json:"porto_id"`
	TecnicoCreatore int64              `json:"tecnico_creatore"` // Chi ha creato la richiesta
	AutomezzoID     *int64             `json:"automezzo_id,omitempty"` // Opzionale
	TargaEsterna    string             `json:"targa_esterna,omitempty"` // Per auto a noleggio o propria
	TipoDurata      TipoDurataPermesso `json:"tipo_durata"`
	DataInizio      time.Time          `json:"data_inizio"`
	DataFine        *time.Time         `json:"data_fine,omitempty"` // NULL se fine_lavori
	Note                 string             `json:"note"`
	DescrizioneIntervento string             `json:"descrizione_intervento"`
	RientroInGiornata    bool               `json:"rientro_in_giornata"`
	EmailInviata    bool               `json:"email_inviata"`
	DataInvioEmail  *time.Time         `json:"data_invio_email,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
	// Campi virtuali
	NomeNave        string             `json:"nome_nave,omitempty"`
	NomePorto       string             `json:"nome_porto,omitempty"`
	NomeTecnico     string             `json:"nome_tecnico,omitempty"`
}

// TecnicoPermesso associa tecnici a una richiesta permesso
type TecnicoPermesso struct {
	ID                  int64 `json:"id"`
	RichiestaPermessoID int64 `json:"richiesta_permesso_id"`
	TecnicoID           int64 `json:"tecnico_id"`
}

// RapportoIntervento rappresenta un rapporto di intervento
type RapportoIntervento struct {
	ID              int64        `json:"id"`
	NaveID          int64        `json:"nave_id"`
	PortoID         int64        `json:"porto_id"`
	Tipo            TipoProdotto `json:"tipo"` // wifi o gsm
	DataIntervento  time.Time    `json:"data_intervento"`
	Descrizione     string       `json:"descrizione"`
	Note            string       `json:"note"`
	DDTGenerato     bool         `json:"ddt_generato"`
	NumeroDDT       string       `json:"numero_ddt,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
	// Campi virtuali
	NomeNave        string       `json:"nome_nave,omitempty"`
	NomeCompagnia   string       `json:"nome_compagnia,omitempty"`
	NomePorto       string       `json:"nome_porto,omitempty"`
}

// TecnicoRapporto associa tecnici a un rapporto intervento
type TecnicoRapporto struct {
	ID         int64 `json:"id"`
	RapportoID int64 `json:"rapporto_id"`
	TecnicoID  int64 `json:"tecnico_id"`
}

// FotoRapporto rappresenta una foto allegata a un rapporto
type FotoRapporto struct {
	ID         int64     `json:"id"`
	RapportoID int64     `json:"rapporto_id"`
	FilePath   string    `json:"file_path"`
	Descrizione string   `json:"descrizione"`
	CreatedAt  time.Time `json:"created_at"`
}

// MaterialeRapporto rappresenta il materiale utilizzato in un intervento
type MaterialeRapporto struct {
	ID         int64 `json:"id"`
	RapportoID int64 `json:"rapporto_id"`
	ProdottoID int64 `json:"prodotto_id"`
	Quantita   int   `json:"quantita"`
	// Campi virtuali
	NomeProdotto string `json:"nome_prodotto,omitempty"`
	CodiceProdotto string `json:"codice_prodotto,omitempty"`
}

// Trasferta rappresenta una trasferta di un tecnico
type Trasferta struct {
	ID            int64     `json:"id"`
	TecnicoID     int64     `json:"tecnico_id"`
	RapportoID           *int64    `json:"rapporto_id,omitempty"` // Collegamento opzionale al rapporto
	RichiestaPermessoID *int64    `json:"richiesta_permesso_id,omitempty"` // Collegamento alla richiesta permesso
	Destinazione  string    `json:"destinazione"`
	DataPartenza  time.Time `json:"data_partenza"`
	DataRientro   time.Time `json:"data_rientro"`
	Pernottamento bool      `json:"pernottamento"` // true = trasferta con notti fuori
	NumeroNotti   int       `json:"numero_notti"`
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	// Campi virtuali
	NomeTecnico   string    `json:"nome_tecnico,omitempty"`
}

// TipoSpesa indica il tipo di spesa
type TipoSpesa string

const (
	SpesaCarburante TipoSpesa = "carburante"
	SpesaHotel      TipoSpesa = "hotel"
	SpesaPranzo     TipoSpesa = "pranzo"
	SpesaCena       TipoSpesa = "cena"
	SpesaMateriali  TipoSpesa = "materiali"
	SpesaVarie      TipoSpesa = "varie"
)

// MetodoPagamento indica come è stata pagata la spesa
type MetodoPagamento string

const (
	PagamentoCartaAziendale MetodoPagamento = "carta_aziendale"
	PagamentoTecnico        MetodoPagamento = "tecnico" // Da rimborsare
)

// NotaSpesa rappresenta una singola voce di spesa
type NotaSpesa struct {
	ID              int64           `json:"id"`
	TecnicoID       int64           `json:"tecnico_id"`
	TrasfertaID     *int64          `json:"trasferta_id,omitempty"` // Collegamento opzionale alla trasferta
	Data            time.Time       `json:"data"`
	TipoSpesa       TipoSpesa       `json:"tipo_spesa"`
	Descrizione     string          `json:"descrizione"`
	Importo         float64         `json:"importo"`
	MetodoPagamento MetodoPagamento `json:"metodo_pagamento"`
	RicevutaPath    string          `json:"ricevuta_path,omitempty"` // Path alla foto/scan della ricevuta
	Note            string          `json:"note"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	// Campi virtuali
	NomeTecnico     string          `json:"nome_tecnico,omitempty"`
}

// TipoDDT indica il tipo di DDT
type TipoDDT string

const (
	DDTIntervento  TipoDDT = "intervento"  // DDT legato a un rapporto intervento
	DDTSpedizione  TipoDDT = "spedizione"  // DDT per spedizione diretta alla nave
)

// DDT rappresenta un documento di trasporto
type DDT struct {
	ID            int64     `json:"id"`
	Numero        string    `json:"numero"`
	TipoDDT       TipoDDT   `json:"tipo_ddt"`               // intervento o spedizione
	RapportoID    *int64    `json:"rapporto_id,omitempty"`  // Opzionale, solo per DDT intervento
	NaveID        int64     `json:"nave_id"`
	CompagniaID   int64     `json:"compagnia_id"`
	PortoID       *int64    `json:"porto_id,omitempty"`     // Porto di destinazione (per spedizioni)
	Destinatario  string    `json:"destinatario,omitempty"` // Nome destinatario per spedizione
	Indirizzo     string    `json:"indirizzo,omitempty"`    // Indirizzo spedizione
	Vettore       string    `json:"vettore,omitempty"`      // Corriere/vettore usato
	DataEmissione time.Time `json:"data_emissione"`
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
	// Campi virtuali
	NomeNave      string    `json:"nome_nave,omitempty"`
	NomeCompagnia string    `json:"nome_compagnia,omitempty"`
	NomePorto     string    `json:"nome_porto,omitempty"`
}

// RigaDDT rappresenta una riga/articolo del DDT
type RigaDDT struct {
	ID          int64   `json:"id"`
	DDTID       int64   `json:"ddt_id"`
	ProdottoID  int64   `json:"prodotto_id"`
	Quantita    float64 `json:"quantita"`
	Descrizione string  `json:"descrizione,omitempty"` // Descrizione aggiuntiva
	// Campi virtuali
	NomeProdotto   string `json:"nome_prodotto,omitempty"`
	CodiceProdotto string `json:"codice_prodotto,omitempty"`
	UnitaMisura    string `json:"unita_misura,omitempty"`
}

// ImpostazioniAzienda rappresenta i dati dell'azienda per documenti e email
type ImpostazioniAzienda struct {
	ID                int64     `json:"id"`
	RagioneSociale    string    `json:"ragione_sociale"`
	PartitaIVA        string    `json:"partita_iva"`
	CodiceFiscale     string    `json:"codice_fiscale"`
	Indirizzo         string    `json:"indirizzo"`
	CAP               string    `json:"cap"`
	Citta             string    `json:"citta"`
	Provincia         string    `json:"provincia"`
	Telefono          string    `json:"telefono"`
	Email             string    `json:"email"`
	PEC               string    `json:"pec"`
	SitoWeb           string    `json:"sito_web"`
	LogoPath          string    `json:"logo_path"`          // Path al file logo
	FirmaEmailPath    string    `json:"firma_email_path"`   // Path immagine firma email
	FirmaEmailTesto   string    `json:"firma_email_testo"`  // Testo firma email (HTML)
	IBAN              string    `json:"iban"`
	Banca             string    `json:"banca"`
	CodiceSDI         string    `json:"codice_sdi"`         // Codice destinatario fatturazione elettronica
	Note              string    `json:"note"`
	// Impostazioni SMTP
	SMTPServer        string    `json:"smtp_server"`
	SMTPPort          int       `json:"smtp_port"`
	SMTPUser          string    `json:"smtp_user"`
	SMTPPassword      string    `json:"smtp_password"`
	SMTPFromName      string    `json:"smtp_from_name"`
	EmailFoglioTrasferte string    `json:"email_foglio_trasferte"` // Email destinatari foglio trasferte
	EmailNotaSpese       string    `json:"email_nota_spese"`       // Email destinatari nota spese
	UpdatedAt         time.Time `json:"updated_at"`
}

// OrarioNave rappresenta un singolo movimento/tratta di una nave
type OrarioNave struct {
	ID                int64     `json:"id"`
	NaveID            int64     `json:"nave_id"`
	Data              time.Time `json:"data"`
	PortoPartenzaID   *int64    `json:"porto_partenza_id,omitempty"`
	PortoArrivoID     *int64    `json:"porto_arrivo_id,omitempty"`
	PortoPartenzaNome string    `json:"porto_partenza_nome"`
	PortoArrivoNome   string    `json:"porto_arrivo_nome"`
	OraPartenza       string    `json:"ora_partenza"`
	OraArrivo         string    `json:"ora_arrivo"`
	Note              string    `json:"note"`
	Fonte             string    `json:"fonte"` // "manuale", "xlsx_corsica", etc.
	CreatedAt         time.Time `json:"created_at"`
	// Campi virtuali
	NomeNave          string    `json:"nome_nave,omitempty"`
}

// SostaNave rappresenta una sosta programmata di una nave in un porto
type SostaNave struct {
	ID           int64     `json:"id"`
	NaveID       int64     `json:"nave_id"`
	PortoID      *int64    `json:"porto_id,omitempty"`
	PortoNome    string    `json:"porto_nome"`
	DataInizio   time.Time `json:"data_inizio"`
	DataFine     *time.Time `json:"data_fine,omitempty"`
	OraArrivo    string    `json:"ora_arrivo"`
	OraPartenza  string    `json:"ora_partenza"`
	Motivo       string    `json:"motivo"`
	Note         string    `json:"note"`
	Fonte        string    `json:"fonte"`
	CreatedAt    time.Time `json:"created_at"`
	// Campi virtuali
	NomeNave     string    `json:"nome_nave,omitempty"`
	NomePorto    string    `json:"nome_porto,omitempty"`
}

// UploadOrari rappresenta un file orari caricato (es. XLSX Corsica Ferries)
type UploadOrari struct {
	ID           int64     `json:"id"`
	CompagniaID  int64     `json:"compagnia_id"`
	NomeFile     string    `json:"nome_file"`
	FilePath     string    `json:"file_path"`
	DataUpload   time.Time `json:"data_upload"`
	CaricatoDa   int64     `json:"caricato_da"`
	Note         string    `json:"note"`
	Attivo       bool      `json:"attivo"`
	// Campi virtuali
	NomeCompagnia string   `json:"nome_compagnia,omitempty"`
	NomeUtente    string   `json:"nome_utente,omitempty"`
}

// ============================================
// APPARATI NAVE
// ============================================

// ApparatoNave rappresenta un dispositivo di rete a bordo
type ApparatoNave struct {
	ID            int64
	NaveID        int64
	Nome          string
	Tipo          string
	IP            string
	MAC           string
	Vendor        string
	Modello       string
	Firmware      string
	Location      string
	SNMPCommunity string
	SSHUser       string
	SSHPass       string
	SSHPort       int
	HTTPUser      string
	HTTPPass      string
	HTTPPort      int
	HTTPSEnabled  bool
	Note          string
	UltimoCheck   string
	Stato         string
	CreatedAt     string
	UpdatedAt     string
	// Campi join
	NomeNave      string
	TipoIcona     string
	TipoColore    string
}

// TipoApparato rappresenta una categoria di dispositivo
type TipoApparato struct {
	ID     int64
	Nome   string
	Icona  string
	Colore string
}

// ObserviumConfig configurazione Observium per nave
type ObserviumConfig struct {
	IP           string
	HTTPUser     string
	HTTPPass     string
	SSHUser      string
	SSHPass      string
	SSHPort      int
	SNMPCommunity string
}

// DeviceDiscovery risultato discovery
type DeviceDiscovery struct {
	Hostname  string
	IP        string
	MAC       string
	Vendor    string
	Type      string
	SysDescr  string
	Location  string
	Status    string
}
