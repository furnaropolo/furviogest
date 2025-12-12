package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"furviogest/internal/auth"
	"furviogest/internal/database"
	"furviogest/internal/handlers"
	"furviogest/internal/middleware"
)

func main() {
	// Flag per configurazione
	port := flag.Int("port", 8080, "Porta del server")
	dbPath := flag.String("db", "", "Percorso del database SQLite")
	flag.Parse()

	// Determina la directory base del progetto
	execPath, err := os.Executable()
	if err != nil {
		log.Fatal("Errore determinazione percorso eseguibile:", err)
	}
	baseDir := filepath.Dir(filepath.Dir(filepath.Dir(execPath)))

	// Se eseguito con go run, usa la directory corrente
	if _, err := os.Stat(filepath.Join(baseDir, "web")); os.IsNotExist(err) {
		baseDir, _ = os.Getwd()
	}

	// Percorso database di default
	if *dbPath == "" {
		*dbPath = filepath.Join(baseDir, "data", "furviogest.db")
	}

	// Inizializza il database
	log.Println("Inizializzazione database:", *dbPath)
	if err := database.InitDB(*dbPath); err != nil {
		log.Fatal("Errore inizializzazione database:", err)
	}
	defer database.CloseDB()

	// Crea utente admin predefinito
	if err := database.CreateDefaultAdmin(auth.HashPassword); err != nil {
		log.Println("Attenzione: errore creazione admin predefinito:", err)
	}

	// Crea tabelle calendario trasferte
	if err := database.AddCalendarioTables(); err != nil {
		log.Println("Attenzione: errore creazione tabelle calendario:", err)
	}

	// Crea tabelle monitoraggio rete
	if err := database.AddMonitoringTables(); err != nil {
		log.Println("Attenzione: errore creazione tabelle monitoraggio:", err)
	}

	// Crea tabella clienti
	if err := database.AddClientiTable(); err != nil {
		log.Println("Attenzione: errore creazione tabella clienti:", err)
	}

	// Crea tabelle DDT uscita
	if err := database.AddDDTUscitaTable(); err != nil {
		log.Println("Attenzione: errore creazione tabelle DDT uscita:", err)
	}

	// Inizializza i template
	templatesDir := filepath.Join(baseDir, "web", "templates")
	log.Println("Caricamento templates da:", templatesDir)
	if err := handlers.InitTemplates(templatesDir); err != nil {
		log.Fatal("Errore caricamento templates:", err)
	}

	// Avvia routine pulizia sessioni scadute
	auth.StartCleanupRoutine()

	// Avvia scheduler monitoraggio rete
	handlers.StartMonitoringScheduler()

	// Configura il router
	mux := http.NewServeMux()

	// File statici
	staticDir := filepath.Join(baseDir, "web", "static")
	fs := http.FileServer(http.Dir(staticDir))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))
	
	// Directory uploads (root level)
	uploadsRoot := filepath.Join(baseDir, "uploads")
	os.MkdirAll(uploadsRoot, 0755)
	uploadsFs := http.FileServer(http.Dir(uploadsRoot))
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", uploadsFs))

	// Directory data (per logo e altri files)
	dataDir := filepath.Join(baseDir, "data")
	dataFs := http.FileServer(http.Dir(dataDir))
	mux.Handle("/data/", http.StripPrefix("/data/", dataFs))

	// Directory uploads
	uploadsDir := filepath.Join(baseDir, "web", "static", "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		log.Println("Attenzione: impossibile creare directory uploads:", err)
	}

	// Route pubbliche (login/logout)
	mux.HandleFunc("/login", handlers.LoginPage)
	mux.HandleFunc("/logout", handlers.Logout)

	// Route protette (richiedono autenticazione)
	mux.Handle("/", middleware.RequireAuth(http.HandlerFunc(handlers.Dashboard)))
	mux.Handle("/cambio-password", middleware.RequireAuth(http.HandlerFunc(handlers.CambioPassword)))

	// Anagrafica Tecnici
	mux.Handle("/tecnici", middleware.RequireAuth(http.HandlerFunc(handlers.ListaTecnici)))
	mux.Handle("/tecnici/nuovo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoTecnico))))
	mux.Handle("/tecnici/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaTecnico))))
	mux.Handle("/tecnici/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaTecnico))))

	// Anagrafica Fornitori
	mux.Handle("/fornitori", middleware.RequireAuth(http.HandlerFunc(handlers.ListaFornitori)))
	mux.Handle("/fornitori/nuovo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoFornitore))))
	mux.Handle("/fornitori/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaFornitore))))
	mux.Handle("/fornitori/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaFornitore))))
	mux.Handle("/api/verifica-piva", middleware.RequireAuth(http.HandlerFunc(handlers.APIVerificaPIVA)))
	mux.Handle("/api/fornitore/info-eliminazione", middleware.RequireAuth(http.HandlerFunc(handlers.APIInfoEliminazioneFornitore)))

	// Anagrafica Porti
	mux.Handle("/porti", middleware.RequireAuth(http.HandlerFunc(handlers.ListaPorti)))
	mux.Handle("/porti/nuovo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoPorto))))
	mux.Handle("/porti/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaPorto))))
	mux.Handle("/porti/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaPorto))))

	// Anagrafica Automezzi
	mux.Handle("/automezzi", middleware.RequireAuth(http.HandlerFunc(handlers.ListaAutomezzi)))
	mux.Handle("/automezzi/nuovo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoAutomezzo))))
	mux.Handle("/automezzi/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaAutomezzo))))
	mux.Handle("/automezzi/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaAutomezzo))))

	// Anagrafica Compagnie
	mux.Handle("/compagnie", middleware.RequireAuth(http.HandlerFunc(handlers.ListaCompagnie)))
	mux.Handle("/compagnie/nuovo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovaCompagnia))))
	mux.Handle("/compagnie/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaCompagnia))))
	mux.Handle("/compagnie/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaCompagnia))))
	mux.Handle("/compagnie/logo/", http.HandlerFunc(handlers.ServeCompagniaLogo))
	mux.Handle("/navi/foto/", http.HandlerFunc(handlers.ServeNaveFoto))

	// Anagrafica Navi
	// Anagrafica Clienti
	mux.Handle("/clienti", middleware.RequireAuth(http.HandlerFunc(handlers.ListaClienti)))
	mux.Handle("/clienti/nuovo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoCliente))))
	mux.Handle("/clienti/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaCliente))))
	mux.Handle("/clienti/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaCliente))))
	mux.Handle("/navi", middleware.RequireAuth(http.HandlerFunc(handlers.ListaNavi)))
	mux.Handle("/navi/nuovo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovaNave))))
	mux.Handle("/navi/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaNave))))
	mux.Handle("/navi/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaNave))))

	// Magazzino Prodotti
	mux.Handle("/magazzino", middleware.RequireAuth(http.HandlerFunc(handlers.ListaProdotti)))
	mux.Handle("/magazzino/nuovo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoProdotto))))
	mux.Handle("/magazzino/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaProdotto))))
	mux.Handle("/magazzino/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaProdotto))))
	mux.Handle("/magazzino/movimenti/", middleware.RequireAuth(http.HandlerFunc(handlers.ListaMovimenti)))
	// DDT Entrata
	mux.Handle("/magazzino/movimento/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoMovimento))))
	// DDT/Fatture (registro documenti acquisto)
	mux.Handle("/ddt-fatture", middleware.RequireAuth(http.HandlerFunc(handlers.ListaDDTFatture)))
	mux.Handle("/ddt-fatture/nuovo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoDDTFattura))))
	mux.Handle("/ddt-fatture/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaDDTFattura))))
	mux.Handle("/ddt-fatture/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaDDTFattura))))
	mux.Handle("/api/ddt-fatture/info-eliminazione", middleware.RequireAuth(http.HandlerFunc(handlers.APIInfoEliminazioneDDTFattura)))
	mux.Handle("/api/ddt-fatture/cerca", middleware.RequireAuth(http.HandlerFunc(handlers.APICercaDDTFatture)))
	// Movimenti acquisto
	mux.Handle("/magazzino/movimento-acquisto/aggiungi", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.AggiungiMovimentoAcquisto))))
	mux.Handle("/magazzino/movimento-acquisto/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaMovimentoAcquisto))))
	mux.Handle("/api/prodotto/dettaglio", middleware.RequireAuth(http.HandlerFunc(handlers.APIDettaglioProdotto)))
	// Archivio PDF
	mux.Handle("/archivio-pdf", middleware.RequireAuth(http.HandlerFunc(handlers.ListaArchivioPDF)))
	mux.Handle("/archivio-pdf/nuovo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoArchivioPDF))))
	mux.Handle("/archivio-pdf/download/", middleware.RequireAuth(http.HandlerFunc(handlers.DownloadArchivioPDF)))
	mux.Handle("/archivio-pdf/visualizza/", middleware.RequireAuth(http.HandlerFunc(handlers.VisualizzaArchivioPDF)))
	mux.Handle("/archivio-pdf/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaArchivioPDF))))

	// Impostazioni Azienda
	mux.Handle("/impostazioni", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ImpostazioniAziendaHandler))))
	mux.Handle("/impostazioni/elimina-logo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaLogo))))
	mux.Handle("/impostazioni/elimina-firma", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaFirmaEmail))))
	mux.Handle("/azienda/logo", middleware.RequireAuth(http.HandlerFunc(handlers.ServeLogoAzienda)))
	mux.Handle("/azienda/firma", middleware.RequireAuth(http.HandlerFunc(handlers.ServeFirmaEmail)))

	// Permessi Accesso Porto
	mux.Handle("/permessi", middleware.RequireAuth(http.HandlerFunc(handlers.ListaPermessi)))
	mux.Handle("/permessi/nuovo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoPermesso))))
	mux.Handle("/permessi/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaPermesso))))
	mux.Handle("/permessi/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaPermesso))))
	mux.Handle("/permessi/dettaglio/", middleware.RequireAuth(http.HandlerFunc(handlers.DettaglioPermesso)))
	mux.Handle("/permessi/anteprima-email/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.AnteprimaEmailPermesso))))
	mux.Handle("/permessi/invia-email/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.InviaEmailPermesso))))
	mux.Handle("/permessi/download-eml/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.DownloadEMLPermesso))))

	// Dettaglio Nave e Orari
	mux.Handle("/navi/dettaglio/", middleware.RequireAuth(http.HandlerFunc(handlers.DettaglioNave)))
	mux.Handle("/navi/orario/nuovo/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoOrario))))
	mux.Handle("/navi/orario/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaOrario))))
	mux.Handle("/navi/sosta/nuovo/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovaSosta))))
	mux.Handle("/navi/sosta/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaSosta))))
	
	// Upload Orari Corsica Ferries
	mux.Handle("/orari/upload", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.UploadOrariPage))))

	// Gestione Rete Nave (AC, Switch, AP)
	mux.Handle("/navi/rete/", middleware.RequireAuth(http.HandlerFunc(handlers.GestioneReteNave)))
	mux.Handle("/navi/ac/salva/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.SalvaAccessController))))
	mux.Handle("/navi/ac/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaAccessController))))
	mux.Handle("/navi/switch/nuovo/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoSwitch))))
	mux.Handle("/navi/switch/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaSwitch))))
	mux.Handle("/navi/switch/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaSwitch))))
	
	// API Monitoraggio Rete
	mux.Handle("/api/rete/scan-ap", middleware.RequireAuth(http.HandlerFunc(handlers.APIScanAccessPoints)))
	mux.Handle("/api/rete/backup-config", middleware.RequireAuth(http.HandlerFunc(handlers.APIBackupConfig)))
	mux.Handle("/api/rete/scan-lldp", middleware.RequireAuth(http.HandlerFunc(handlers.APIScanLLDP)))
	mux.Handle("/api/rete/scan-ports", middleware.RequireAuth(http.HandlerFunc(handlers.APIScanPorts)))
	mux.Handle("/api/guasti-nave", middleware.RequireAuth(http.HandlerFunc(handlers.APIGuastiNave)))
	mux.Handle("/api/rete/switch-version", middleware.RequireAuth(http.HandlerFunc(handlers.APIGetSwitchVersion)))
	mux.Handle("/api/rete/ap-fault", middleware.RequireAuth(http.HandlerFunc(handlers.APIGetAPFault)))
	mux.Handle("/api/rete/ac-version", middleware.RequireAuth(http.HandlerFunc(handlers.APIGetACVersion)))
	mux.Handle("/api/rete/download-config/", middleware.RequireAuth(http.HandlerFunc(handlers.APIDownloadConfig)))
	mux.Handle("/api/rete/test-ssh", middleware.RequireAuth(http.HandlerFunc(handlers.APITestSSH)))
	mux.Handle("/api/rete/export-ap-csv", middleware.RequireAuth(http.HandlerFunc(handlers.APIExportAPCSV)))

	// Uffici
	mux.Handle("/uffici", middleware.RequireAuth(http.HandlerFunc(handlers.ListaUffici)))
	mux.Handle("/uffici/nuovo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoUfficio))))
	mux.Handle("/uffici/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaUfficio))))
	mux.Handle("/uffici/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaUfficio))))
	mux.Handle("/uffici/rete/", middleware.RequireAuth(http.HandlerFunc(handlers.GestioneReteUfficio)))
	mux.Handle("/uffici/ac/salva/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.SalvaACUfficio))))
	mux.Handle("/uffici/ac/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaACUfficio))))
	mux.Handle("/uffici/switch/nuovo/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoSwitchUfficio))))
	mux.Handle("/uffici/switch/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaSwitchUfficio))))
	mux.Handle("/uffici/switch/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaSwitchUfficio))))

	// Sale Server
	mux.Handle("/sale-server", middleware.RequireAuth(http.HandlerFunc(handlers.ListaSaleServer)))
	mux.Handle("/sale-server/nuova", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovaSalaServer))))
	mux.Handle("/sale-server/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaSalaServer))))
	mux.Handle("/sale-server/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaSalaServer))))
	mux.Handle("/sale-server/rete/", middleware.RequireAuth(http.HandlerFunc(handlers.GestioneReteSalaServer)))
	mux.Handle("/sale-server/switch/nuovo/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoSwitchSalaServer))))
	mux.Handle("/sale-server/switch/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaSwitchSalaServer))))
	mux.Handle("/sale-server/switch/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaSwitchSalaServer))))

	// API Backup Uffici e Sale Server
	mux.Handle("/api/rete/backup-config-ufficio", middleware.RequireAuth(http.HandlerFunc(handlers.APIBackupConfigUfficio)))
	mux.Handle("/api/rete/download-backup-ufficio/", middleware.RequireAuth(http.HandlerFunc(handlers.APIDownloadBackupUfficio)))
	mux.Handle("/api/rete/elimina-backup-ufficio/", middleware.RequireAuth(http.HandlerFunc(handlers.APIEliminaBackupUfficio)))
	mux.Handle("/api/uffici/scan-ap/", middleware.RequireAuth(http.HandlerFunc(handlers.APIScanAPUfficio)))
	mux.Handle("/api/uffici/scan-ports", middleware.RequireAuth(http.HandlerFunc(handlers.APIScanPortsUfficio)))
	mux.Handle("/api/sale-server/scan-ports", middleware.RequireAuth(http.HandlerFunc(handlers.APIScanPortsSalaServer)))

	// Attrezzi e Consumabili
	mux.Handle("/attrezzi", middleware.RequireAuth(http.HandlerFunc(handlers.ListaAttrezzi)))
	mux.Handle("/attrezzi/nuovo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoAttrezzo))))
	mux.Handle("/attrezzi/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaAttrezzo))))
	mux.Handle("/attrezzi/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaAttrezzo))))
	mux.Handle("/attrezzi/movimento/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.MovimentoAttrezzoHandler))))
	mux.Handle("/attrezzi/storico/", middleware.RequireAuth(http.HandlerFunc(handlers.StoricoAttrezzo)))

	// Route Amministrazione
	mux.Handle("/amministrazione", middleware.RequireAuth(middleware.RequireTecnicoOrAmministrazione(http.HandlerFunc(handlers.DashboardAmministrazione))))
	mux.Handle("/amministrazione/magazzino", middleware.RequireAuth(middleware.RequireTecnicoOrAmministrazione(http.HandlerFunc(handlers.GiacenzaMagazzino))))
	mux.Handle("/amministrazione/magazzino/export", middleware.RequireAuth(middleware.RequireTecnicoOrAmministrazione(http.HandlerFunc(handlers.ExportMagazzinoCSV))))
	mux.Handle("/amministrazione/rapporti", middleware.RequireAuth(middleware.RequireTecnicoOrAmministrazione(http.HandlerFunc(handlers.ListaRapportiAmministrazione))))
	mux.Handle("/amministrazione/note-spese", middleware.RequireAuth(middleware.RequireTecnicoOrAmministrazione(http.HandlerFunc(handlers.NoteSpeseAmministrazione))))
	mux.Handle("/amministrazione/note-spese/export", middleware.RequireAuth(middleware.RequireTecnicoOrAmministrazione(http.HandlerFunc(handlers.ExportNoteSpeseCSV))))
	mux.Handle("/amministrazione/trasferte", middleware.RequireAuth(middleware.RequireTecnicoOrAmministrazione(http.HandlerFunc(handlers.RiepilogoTrasferteAmministrazione))))
	mux.Handle("/amministrazione/trasferte/export", middleware.RequireAuth(middleware.RequireTecnicoOrAmministrazione(http.HandlerFunc(handlers.ExportTrasferteCSV))))
	mux.Handle("/amministrazione/riepilogo", middleware.RequireAuth(middleware.RequireTecnicoOrAmministrazione(http.HandlerFunc(handlers.RiepilogoMensile))))
	mux.Handle("/amministrazione/ddt", middleware.RequireAuth(middleware.RequireTecnicoOrAmministrazione(http.HandlerFunc(handlers.DDTAmministrazione))))
	mux.Handle("/amministrazione/ddt/export", middleware.RequireAuth(middleware.RequireTecnicoOrAmministrazione(http.HandlerFunc(handlers.ExportDDTCSV))))
	mux.Handle("/amministrazione/ddt/", middleware.RequireAuth(middleware.RequireTecnicoOrAmministrazione(http.HandlerFunc(handlers.DettaglioDDTAmministrazione))))

	// Rapporti Intervento
	mux.Handle("/rapporti", middleware.RequireAuth(http.HandlerFunc(handlers.ListaRapporti)))
	mux.Handle("/rapporti/nuovo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoRapporto))))
	mux.Handle("/rapporti/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaRapporto))))
	mux.Handle("/rapporti/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaRapportoDefinitivo))))
	mux.Handle("/rapporti/dettaglio/", middleware.RequireAuth(http.HandlerFunc(handlers.DettaglioRapporto)))
	mux.Handle("/rapporti/pdf/", middleware.RequireAuth(http.HandlerFunc(handlers.RapportoPDF)))
	mux.Handle("/rapporti/download-pdf/", middleware.RequireAuth(http.HandlerFunc(handlers.RapportoDownloadPDF)))
	mux.Handle("/rapporti/foto/upload", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.UploadFotoRapporto))))
	mux.Handle("/rapporti/foto/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaFotoRapporto))))
	mux.Handle("/navi/storico/", middleware.RequireAuth(http.HandlerFunc(handlers.StoricoInterventiNave)))

	// Trasferte
	mux.Handle("/trasferte", middleware.RequireAuth(http.HandlerFunc(handlers.ListaTrasferte)))
	mux.Handle("/trasferte/nuovo", middleware.RequireAuth(http.HandlerFunc(handlers.NuovaTrasferta)))
	mux.Handle("/trasferte/modifica/", middleware.RequireAuth(http.HandlerFunc(handlers.ModificaTrasferta)))
	mux.Handle("/trasferte/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaTrasferta))))

	// Note Spese Tecnici
	mux.Handle("/note-spese", middleware.RequireAuth(http.HandlerFunc(handlers.ListaNoteSpese)))
	mux.Handle("/note-spese/nuovo", middleware.RequireAuth(http.HandlerFunc(handlers.NuovaNotaSpesa)))
	mux.Handle("/note-spese/modifica/", middleware.RequireAuth(http.HandlerFunc(handlers.ModificaNotaSpesa)))
	mux.Handle("/note-spese/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaNotaSpesa))))

	// DDT Uscita Magazzino
	mux.Handle("/ddt-uscita", middleware.RequireAuth(http.HandlerFunc(handlers.ListaDDTUscita)))
	mux.Handle("/ddt-uscita/nuovo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoDDTUscita))))
	mux.Handle("/ddt-uscita/dettaglio/", middleware.RequireAuth(http.HandlerFunc(handlers.DettaglioDDTUscita)))
	mux.Handle("/ddt-uscita/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaDDTUscita))))
	mux.Handle("/ddt-uscita/annulla/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.AnnullaDDTUscita))))
	mux.Handle("/ddt-uscita/riga/aggiungi", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.AggiungiRigaDDTUscita))))
	mux.Handle("/ddt-uscita/riga/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.RimuoviRigaDDTUscita))))
	mux.Handle("/api/ddt-uscita/cerca-prodotti", middleware.RequireAuth(http.HandlerFunc(handlers.APICercaProdottiDDT)))
	mux.Handle("/ddt-uscita/pdf/", middleware.RequireAuth(http.HandlerFunc(handlers.PDFDDTUscita)))

	// DDT Tecnici
	mux.Handle("/ddt", middleware.RequireAuth(http.HandlerFunc(handlers.ListaDDT)))
	mux.Handle("/ddt/nuovo", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.NuovoDDT))))
	mux.Handle("/ddt/modifica/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.ModificaDDT))))
	mux.Handle("/ddt/dettaglio/", middleware.RequireAuth(http.HandlerFunc(handlers.DettaglioDDT)))
	mux.Handle("/ddt/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaDDT))))
	mux.Handle("/ddt/riga/aggiungi", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.AggiungiRigaDDT))))
	mux.Handle("/ddt/riga/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.RimuoviRigaDDT))))
	mux.Handle("/ddt/genera-numero", middleware.RequireAuth(http.HandlerFunc(handlers.GeneraNumDDT)))

	// Foglio Mensile
	mux.Handle("/foglio-mensile", middleware.RequireAuth(http.HandlerFunc(handlers.FoglioMensile)))

	// Calendario Trasferte
	mux.Handle("/calendario-trasferte", middleware.RequireAuth(http.HandlerFunc(handlers.CalendarioTrasferte)))
	mux.Handle("/api/calendario/giornata", middleware.RequireAuth(http.HandlerFunc(handlers.APIDettaglioGiornata)))
	mux.Handle("/api/calendario/salva-giornata", middleware.RequireAuth(http.HandlerFunc(handlers.APISalvaGiornata)))
	mux.Handle("/api/calendario/salva-spesa", middleware.RequireAuth(http.HandlerFunc(handlers.APISalvaSpesa)))
	mux.Handle("/api/calendario/elimina-spesa", middleware.RequireAuth(http.HandlerFunc(handlers.APIEliminaSpesa)))
	mux.Handle("/api/calendario/elimina-giornata", middleware.RequireAuth(http.HandlerFunc(handlers.APIEliminaGiornata)))
	mux.Handle("/stampa-trasferte", middleware.RequireAuth(http.HandlerFunc(handlers.StampaTrasferte)))
	mux.Handle("/stampa-note-spese", middleware.RequireAuth(http.HandlerFunc(handlers.StampaNoteSpese)))
	mux.Handle("/email-trasferte", middleware.RequireAuth(http.HandlerFunc(handlers.InviaEmailTrasferte)))
	mux.Handle("/email-note-spese", middleware.RequireAuth(http.HandlerFunc(handlers.InviaEmailNoteSpese)))
	mux.Handle("/download-pdf-trasferte", middleware.RequireAuth(http.HandlerFunc(handlers.DownloadPDFTrasferte)))
	mux.Handle("/download-pdf-note-spese", middleware.RequireAuth(http.HandlerFunc(handlers.DownloadPDFNoteSpese)))
	mux.Handle("/api/navi-compagnia", middleware.RequireAuth(http.HandlerFunc(handlers.APINaviCompagnia)))
	// Guasti Nave
	mux.Handle("/guasti-nave", middleware.RequireAuth(http.HandlerFunc(handlers.ListaNaviGuasti)))
	mux.Handle("/guasti-nave/", middleware.RequireAuth(http.HandlerFunc(handlers.GuastiNave)))
	mux.Handle("/guasti-nave/nuovo/", middleware.RequireAuth(http.HandlerFunc(handlers.NuovoGuasto)))
	mux.Handle("/guasti-nave/modifica/", middleware.RequireAuth(http.HandlerFunc(handlers.ModificaGuasto)))
	mux.Handle("/guasti-nave/elimina/", middleware.RequireAuth(middleware.RequireTecnico(http.HandlerFunc(handlers.EliminaGuasto))))
	mux.Handle("/guasti-nave/storico", middleware.RequireAuth(http.HandlerFunc(handlers.StoricoGuasti)))

	// Avvia il server
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Server FurvioGest avviato su http://localhost%s", addr)
	log.Println("Credenziali predefinite: admin / admin")
	log.Println("IMPORTANTE: Cambiare la password al primo accesso!")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal("Errore avvio server:", err)
	}
}
