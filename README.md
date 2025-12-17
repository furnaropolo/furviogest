# FurvioGest

Sistema gestionale completo per il monitoraggio di reti navali e la gestione operativa di interventi tecnici.

## Descrizione

FurvioGest è un'applicazione web sviluppata in Go per la gestione delle operazioni IT su navi Ro-Pax. Nasce dall'esigenza di centralizzare e semplificare le attività quotidiane di un IT System Administrator che gestisce infrastrutture di rete su più imbarcazioni.

## Funzionalita

- **Monitoraggio Reti Navali** - Dashboard per il controllo dello stato delle reti WiFi/GSM a bordo
- **Gestione Magazzino** - Inventario hardware e software con tracking delle scorte
- **Gestione Interventi** - Registrazione e tracciamento degli interventi tecnici effettuati
- **Gestione Trasferte** - Pianificazione e storico delle trasferte sulle navi
- **Note Spese** - Gestione delle spese di viaggio e rimborsi

## Tecnologie

- **Backend**: Go
- **Database**: SQLite
- **Frontend**: HTML/CSS/JavaScript
- **Template Engine**: Go html/template

## Installazione

```bash
# Clona il repository
git clone https://github.com/furnaropolo/furviogest.git
cd furviogest

# Compila il progetto
go build -o furviogest ./cmd/server

# Avvia il server
./furviogest
```

Il server sara disponibile su `http://localhost:8080`

## Struttura del Progetto

```
furviogest/
├── cmd/
│   └── server/          # Entry point dell'applicazione
├── internal/
│   ├── handlers/        # HTTP handlers
│   ├── models/          # Modelli dati
│   └── database/        # Gestione database
├── templates/           # Template HTML
├── static/              # File statici (CSS, JS, immagini)
└── data/
    └── furviogest.db    # Database SQLite
```

## Licenza

Questo progetto e rilasciato con licenza MIT.

## Autore

**Francesco Politano**
- GitHub: [@furnaropolo](https://github.com/furnaropolo)
- LinkedIn: [francesco-politano-b9161920](https://linkedin.com/in/francesco-politano-b9161920)
