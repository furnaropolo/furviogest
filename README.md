# FurvioGest

Sistema gestionale per la gestione operativa di interventi tecnici su navi, porti e infrastrutture marittime.

## Caratteristiche

- **Gestione Anagrafiche**: Tecnici, Fornitori, Porti, Automezzi, Compagnie, Navi
- **Magazzino**: Gestione prodotti con movimenti di carico/scarico
- **Rapporti Intervento**: Creazione e gestione rapporti con materiali e foto
- **Trasferte**: Gestione trasferte dei tecnici
- **Note Spese**: Tracciamento spese dei tecnici
- **DDT**: Gestione documenti di trasporto
- **Permessi Porto**: Gestione permessi di accesso ai porti
- **Apparati Nave**: Gestione apparati tecnici con integrazione Observium
- **Attrezzi e Consumabili**: Inventario attrezzi con storico movimenti
- **Amministrazione**: Dashboard per ufficio contabilità con export CSV
- **Foglio Mensile**: Riepilogo mensile attività

## Requisiti

- Go 1.25+
- SQLite3

## Installazione

```bash
# Clona il repository
git clone https://gitlab.ies-italia.it/francesco.politano/furviogest.git
cd furviogest

# Compila
go build -o furviogest ./cmd/server

# Esegui
./furviogest
```

## Configurazione

Il server accetta i seguenti parametri:

| Parametro | Default | Descrizione |
|-----------|---------|-------------|
| `-port` | 8000 | Porta del server HTTP |
| `-db` | data/furviogest.db | Percorso del database SQLite |

## Primo Avvio

Al primo avvio viene creato automaticamente un utente amministratore:
- **Username**: admin
- **Password**: admin

⚠️ **Importante**: Cambiare la password al primo accesso!

## Struttura Progetto

```
furviogest/
├── cmd/
│   └── server/          # Entry point applicazione
├── internal/
│   ├── auth/            # Autenticazione e sessioni
│   ├── database/        # Gestione database SQLite
│   ├── email/           # Invio email
│   ├── handlers/        # Handler HTTP
│   ├── middleware/      # Middleware autenticazione
│   └── models/          # Modelli dati
├── migrations/          # Script migrazione database
├── web/
│   ├── static/          # File statici (CSS, JS, immagini)
│   └── templates/       # Template HTML
└── data/                # Database SQLite
```

## Ruoli Utente

- **Tecnico**: Accesso completo a tutte le funzionalità
- **Amministrazione**: Accesso in sola lettura per contabilità

## Licenza

Proprietario - IES Italia

## Autore

IES Italia - 2024
