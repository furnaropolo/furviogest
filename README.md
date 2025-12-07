# FurvioGest

Sistema gestionale web per la gestione di interventi di manutenzione WiFi/GSM su navi.

## Descrizione

FurvioGest è un'applicazione web sviluppata per gestire tutte le attività legate agli interventi tecnici su imbarcazioni, inclusi:

- **Gestione anagrafiche**: tecnici, fornitori, porti, automezzi, compagnie navali e navi
- **Magazzino**: prodotti con giacenza, movimenti carico/scarico, attrezzi con tracciamento posizione
- **Apparati nave**: gestione router, switch, access point, firewall con credenziali SSH/HTTP
- **Richieste permessi porto**: generazione automatica email con allegati per l'accesso ai porti
- **Calendario trasferte**: pianificazione giornate lavorative con calcolo automatico festivi
- **Note spese**: gestione spese con riepilogo mensile e calcolo rimborsi
- **Rapporti intervento**: documentazione degli interventi effettuati
- **DDT**: generazione documenti di trasporto

## Funzionalità Principali

### Richiesta Permessi Porto
- Creazione richiesta con selezione nave, porto, tecnici e automezzo
- Anteprima email prima dell'invio
- Invio automatico all'agenzia portuale con allegati (documenti tecnici + libretto automezzo)
- Gestione destinatari personalizzata per compagnia

### Calendario Trasferte
- Visualizzazione mensile a griglia (stile foglio presenze)
- Codifica colori per tipo giornata:
  - Bianco = Ufficio
  - Giallo = Trasferta Giornaliera
  - Verde = Trasferta con Pernotto
  - Rosso = Trasferta Festiva
  - Blu = Ferie
  - Viola = Permesso
- Calcolo automatico festivi nazionali italiani

### Note Spese Integrate
- Inserimento spese per giornata (carburante, vitto/alloggio, pedaggi, materiali)
- Distinzione carta aziendale / personale (da rimborsare)
- Riepilogo mensile con totali per categoria
- Stampa/PDF e invio email

## Requisiti

- Go 1.21+
- SQLite3

## Installazione

```bash
# Clona il repository
git clone https://github.com/furnaropolo/furviogest.git
cd furviogest

# Scarica le dipendenze
go mod tidy

# Avvia il server
go run cmd/server/main.go
```

Il server sarà disponibile su http://localhost:8080

## Credenziali Default

- **Username**: admin
- **Password**: admin

## Stack Tecnologico

- **Backend**: Go (Golang)
- **Database**: SQLite
- **Frontend**: HTML/CSS/JavaScript + Go Templates
- **Datepicker**: Flatpickr (locale italiano)
- **Email**: SMTP (compatibile Gmail con App Password)

## Struttura Progetto

```
furviogest/
├── cmd/server/          # Entry point applicazione
├── internal/
│   ├── database/        # Connessione e setup DB
│   ├── handlers/        # Handler HTTP
│   ├── middleware/      # Autenticazione e middleware
│   └── models/          # Strutture dati
├── web/
│   ├── static/          # CSS, JS, immagini
│   └── templates/       # Template HTML
└── data/                # Database e uploads
```

## Documentazione Sviluppo

Per dettagli sullo stato di sviluppo, funzionalità implementate e changelog, vedere [DEVELOPMENT.md](DEVELOPMENT.md).

## Licenza

Progetto privato - Tutti i diritti riservati
