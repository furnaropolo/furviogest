# FurvioGest - Sistema Gestionale Interventi Manutenzione WiFi/GSM

## Accesso
- **URL**: http://localhost:8080
- **Credenziali default**: admin / admin


---

## REGOLE DI SVILUPPO - LEGGERE PRIMA DI QUALSIASI MODIFICA

### Principio Fondamentale: COMPARTIMENTI BLINDATI

Questo progetto segue un approccio a **compartimenti blindati**. Ogni funzionalita e isolata e indipendente.

### Regole INVIOLABILI:

1. **NON MODIFICARE MAI codice funzionante**
   - Se una funzionalita funziona, NON si tocca
   - Nessuna "ottimizzazione" o "refactoring" di codice esistente
   - Nessuna modifica "cosmetica" a file che funzionano

2. **Nuove funzionalita = Nuovo codice**
   - Aggiungere, mai modificare
   - Nuovi handler in file separati se possibile
   - Nuovi template senza toccare quelli esistenti

3. **Test prima e dopo**
   - Prima di modificare: verificare che tutto funzioni
   - Dopo ogni modifica: verificare che TUTTO funzioni ancora
   - Se qualcosa si rompe: ROLLBACK immediato

4. **Un cambiamento alla volta**
   - Una funzionalita per sessione
   - Testare completamente prima di passare ad altro
   - Mai modifiche multiple in parallelo

5. **Database: solo ADD, mai ALTER/DROP**
   - Nuove tabelle: OK
   - Nuove colonne: OK (con ALTER TABLE ADD)
   - Modificare colonne esistenti: VIETATO
   - Eliminare tabelle/colonne: VIETATO

6. **Template: isolare le modifiche**
   - Usare blocchi {{define}} separati
   - CSS/JS aggiuntivo in file separati o in fondo
   - Mai modificare struttura HTML esistente che funziona

### Prima di ogni modifica, chiedersi:

- Questa modifica puo rompere qualcosa che funziona?
- Posso aggiungere invece di modificare?
- Ho testato la funzionalita PRIMA di toccarla?
- Ho un modo per tornare indietro se qualcosa va storto?

7. **RIAVVIARE SEMPRE il server dopo modifiche**
   - Go serve i file statici e template dalla memoria
   - Dopo OGNI modifica a .go, .html, .css, .js: RIAVVIARE
   - Comando: pkill -9 furviogest && ./furviogest
   - Se modifichi .go: ricompilare prima con go build

### In caso di dubbio: NON MODIFICARE

Se non sei sicuro al 100% che una modifica sia sicura, NON farla.
Meglio codice "brutto" che funziona che codice "elegante" che rompe tutto.


## Stato Progetto: ✅ COMPLETATO E TESTATO

Il programma è stato completato e testato con successo, incluso:
- ✅ Test completo di tutte le funzionalità
- ✅ Installazione da zero su Fedora Linux con recupero da backup
- ⏳ Installazione da Windows (in sospeso - previsto installer .exe)

---

## Funzionalità Implementate

### Anagrafiche
- [x] Tecnici (con upload documento identità, configurazione SMTP personale)
- [x] Fornitori (con verifica P.IVA, gestione DDT/Fatture collegati)
- [x] Clienti (ragione sociale, indirizzo, P.IVA, contatti)
- [x] Porti (con dati agenzia: nome, email, telefono)
- [x] Automezzi (con upload libretto)
- [x] Compagnie (con opzione destinatari email, logo, sede legale completa)
- [x] Navi (con email master, direttore macchina, ispettore + config Observium)
- [x] **Disegni Nave**
- [x] **Apparati Nave** - Gestione server/VM per nave (nome, IP, porta, protocollo) con pulsante GUI - Upload multipli PDF (schemi rete, layout AP, cablaggi) per ogni nave
- [x] Uffici (gestione sedi con rete AC/Switch)
- [x] Sale Server (gestione con Switch per backup)

### Magazzino
- [x] Prodotti con gestione giacenza e prezzi
- [x] Movimenti carico/scarico
- [x] Attrezzi con tracciamento posizione (sede/tecnico/nave)
- [x] **DDT/Fatture Entrata** - Registro documenti acquisto con collegamento a fornitore
- [x] **DDT Uscita** - Documenti di trasporto uscita con:
  - Numerazione automatica progressiva per anno
  - Selezione destinatario (cliente/nave)
  - Righe prodotti con scarico automatico giacenza
  - Generazione PDF professionale
  - Gestione stato (bozza/emesso/annullato)
- [x] **Archivio PDF** - Upload e gestione documenti PDF (fatture, DDT, manuali)

### Apparati Nave
- [x] Gestione apparati per nave (router, switch, AP, firewall, ecc.)
- [x] Configurazione credenziali SSH/HTTP/Telnet per ogni apparato
- [x] Scan AP da Access Controller Huawei
- [x] Scan MAC table da switch Huawei/HP
- [x] Backup automatico configurazione switch/AC
- [x] Auto-rilevamento hostname switch
- [x] **Rilevamento automatico licenze AP** - pulsante "Rileva Licenze" per AC Huawei
- [x] Export CSV lista AP

### Richiesta Permessi Porto
- [x] Creazione richiesta con: nave, porto, tecnici, automezzo, date
- [x] Tipo durata: giornaliera, multigiorno, fino a fine lavori
- [x] Campo Rientro in giornata (checkbox) - uso interno
- [x] Descrizione intervento (inclusa nell'email)
- [x] Anteprima email prima dell'invio
- [x] Invio email automatico all'agenzia (e opzionalmente a master/DDM/ispettore)
- [x] Allegati automatici: documenti tecnici + libretto automezzo
- [x] Download file .eml per invio manuale
- [x] Alert guasti nave e AP fault nel dettaglio permesso

### Calendario Trasferte + Note Spese Integrato
- [x] Calendario mensile a griglia (visualizzazione tipo foglio presenze)
- [x] Colori per tipo giornata:
  - Bianco = Ufficio
  - Giallo = Trasferta Giornaliera
  - Verde = Trasferta con Pernotto
  - Rosso = Trasferta Festiva
  - Blu = Ferie
  - Viola = Permesso
- [x] Calcolo automatico festivi (nazionali + Sant'Ambrogio per Milano)
- [x] Click su giorno apre modale per inserire tipo giornata e spese
- [x] Tipi spesa: Carburante, Cibo/Hotel, Pedaggi/Taxi, Materiali, Varie
- [x] Metodo pagamento: Carta aziendale / Carta personale (da rimborsare)
- [x] Riepilogo mensile con totali per categoria
- [x] Conteggio giorni per tipo (ufficio, trasferte, ferie)

### Stampa e Invio Email Documenti
- [x] Anteprima Foglio Trasferte (formato A4) con logo e dati azienda
- [x] Anteprima Nota Spese (formato A4) con logo e dati azienda
- [x] Stampa / Salva PDF diretta da anteprima
- [x] Download PDF diretto
- [x] Invio Email da anteprima

### Rapporti Intervento
- [x] Creazione rapporti con dettaglio lavori
- [x] Collegamento a nave e tecnici
- [x] Upload foto intervento
- [x] Generazione PDF professionale
- [x] Storico interventi per nave

### Segnalazione Guasti Nave
- [x] Lista navi con conteggio guasti attivi e badge gravità
- [x] Pagina guasti per singola nave con card colorate per gravità
- [x] Gestione stato: aperto → preso in carico → risolto
- [x] Selezione tecnico obbligatoria per presa in carico/risoluzione
- [x] Auto-inserimento guasto quando AP va in fault
- [x] Auto-chiusura guasto quando AP torna online
- [x] Storico guasti con filtro date

### Backup e Ripristino
- [x] Backup manuale database (download .zip)
- [x] Ripristino da file .zip (upload)
- [x] Backup automatico programmabile
- [x] Backup su NAS via SMB/CIFS
- [x] Test connessione NAS
- [x] Lista backup locali con download/elimina
- [x] Alert in dashboard se backup fallisce
- [x] **Backup Configurazioni Rete su NAS** - backup notturno config AC/Switch
  - Retention configurabile (default 3 backup)
  - Esclusione automatica navi ferme per lavori
  - UI con riepilogo/modifica/disabilita

### Amministrazione
- [x] Dashboard separata per ruolo "amministrazione"
- [x] Visualizzazione rapporti, note spese, trasferte
- [x] Riepilogo mensile con totali
- [x] Export CSV (magazzino, note spese, trasferte, DDT)

### Impostazioni
- [x] Dati azienda (ragione sociale, indirizzo, P.IVA, ecc.)
- [x] Firma email personalizzabile (HTML)
- [x] Logo azienda
- [x] Configurazione SMTP per invio email
- [x] Email destinatari foglio trasferte e nota spese

### UI/UX
- [x] Datepicker italiano (formato GG/MM/AAAA) su tutti i campi data
- [x] Interfaccia responsive
- [x] Menu contestuale per ruolo (tecnico/amministrazione)
- [x] Bootstrap 5 + Bootstrap Icons
- [x] Accordion collapsabili per liste lunghe (es. navi per compagnia)
- [x] Filtri ricerca nelle liste
- [x] **Menu con emoji colorate** - icone intuitive sotto ogni voce
- [x] **Font Poppins** - tipografia moderna e professionale
- [x] **Pulsanti navigazione** - Indietro/Home su tutte le pagine

---

## Stack Tecnologico

- **Backend**: Go (Golang) 1.21+
- **Database**: SQLite3
- **Frontend**: HTML/CSS/JS + Go Templates
- **CSS Framework**: Bootstrap 5.3
- **Icons**: Bootstrap Icons
- **Datepicker**: Flatpickr (locale italiano)
- **Email**: SMTP (compatibile Gmail con App Password)

---

## Struttura Progetto

```
furviogest/
├── cmd/server/          # Entry point applicazione
├── internal/
│   ├── auth/            # Autenticazione e sessioni
│   ├── database/        # Connessione e setup DB
│   ├── handlers/        # Handler HTTP
│   ├── middleware/      # Autenticazione e middleware
│   └── models/          # Strutture dati
├── web/
│   ├── static/          # CSS, JS, immagini
│   └── templates/       # Template HTML
├── data/                # Database e uploads
├── uploads/             # File caricati (documenti, foto)
└── backups/             # Backup database
```

---

## Changelog

### 2025-12-15 (sessione 17)
- **Backup Configurazioni Rete su NAS - Uffici e Sale Server**:
  - Esteso backup automatico su NAS per includere uffici e sale server
  - Nuove cartelle NAS: config_uffici/ e config_sale_server/
  - Stessa retention configurabile (default 3 backup per apparato)
  - Usa smbclient invece di mount (non richiede privilegi root)
  - Struttura NAS:
    - config_navi/nave_X/ (esistente)
    - config_uffici/ufficio_X/ (NUOVO)
    - config_sale_server/sala_server_X/ (NUOVO)


### 2025-12-14 (sessione 16)
- **Fix Backup Automatico**:
  - Corretto script cron (CRLF -> LF)
  - Corretto percorso log (/var/log -> /home/user/furviogest)
  - Backup notturno ora funzionante
- **Licenze AP su Access Controller**:
  - Sostituito campo Note con Licenze Totali/Utilizzate
  - Pulsante "Rileva Licenze" per rilevamento automatico
  - Comando SSH: display license resource usage
  - Parsing automatico output Huawei (es. 46/48)
- **Backup Configurazioni Rete su NAS**:
  - Opzione backup notturno automatico configurazioni AC/Switch
  - Destinazione: cartella NAS configurata + /config_navi/nave_X/
  - Retention configurabile (default 3 backup per apparato)
  - Esclusione automatica navi ferme per lavori
  - UI coerente con sezione NAS (riepilogo/modifica/disabilita)
- **Navigazione migliorata**:
  - Pulsanti Indietro (←) e Home (🏠) in alto a sinistra su tutte le pagine
  - Nascosti automaticamente nella dashboard
- **Restyling Menu**:
  - Emoji colorate sotto ogni voce di menu
  - Font Poppins professionale
  - Testo maiuscolo grassetto
  - Layout centrato e proporzionato
- **Restyling Dashboard**:
  - Rimossa scritta "Dashboard", solo benvenuto utente
  - Card colorate con gradient (viola, rosa, verde, arancione)
  - Bottoni con emoji al posto di liste link
  - Layout verticale con ombre e bordi arrotondati
  - Miglior contrasto e leggibilità
- **Apparati Nave**:
  - Nuova sezione per gestire server/VM per ogni nave
  - Campi: nome, indirizzo IP, porta, protocollo (HTTP/HTTPS)
  - Pulsante GUI per aprire interfaccia web in nuova finestra
  - Pulsante nella barra navigazione e nella lista navi
- **Emoji UI**:
  - Sostituite icone Bootstrap con emoji colorate
  - Applicato a lista navi e dettaglio nave
- **Disegni Nave**:
  - Nuova tabella disegni_nave per upload multipli per nave
  - Supporto PDF formato A3 (schemi rete, layout AP, cablaggi)
  - Form upload con nome personalizzabile per ogni disegno
  - Lista disegni con visualizzazione e eliminazione
- **Fix Upload Libretto Automezzi**:
  - Corretta gestione upload file per nuovo automezzo e modifica
  - File salvati in /uploads/libretti/


### 2025-12-12 (sessione 13-15) - COMPLETAMENTO PROGETTO
- **Sistema Backup Completo**:
  - Backup manuale con download .zip
  - Ripristino da upload .zip
  - Backup automatico programmabile (giornaliero/settimanale)
  - Integrazione NAS via SMB/CIFS
  - Test connessione NAS con feedback
  - Alert in dashboard se backup fallisce
- **DDT Uscita Completo**:
  - Lista DDT con filtri e ricerca
  - Form creazione con selezione cliente/nave
  - Dettaglio DDT con righe prodotti
  - Aggiunta/rimozione righe con scarico giacenza
  - Stati: bozza, emesso, annullato
  - Generazione PDF professionale
- **Anagrafica Clienti**:
  - CRUD completo
  - Campi: ragione sociale, indirizzo, città, CAP, provincia, P.IVA, CF, telefono, email, note
- **DDT/Fatture Entrata**:
  - Registro documenti acquisto
  - Collegamento a fornitore
  - Collegamento movimenti magazzino
  - Ricerca documenti per numero/fornitore
- **Archivio PDF**:
  - Upload documenti PDF
  - Categorizzazione (fattura, DDT, manuale, altro)
  - Collegamento opzionale a fornitore
  - Download e visualizzazione inline
- **Test Installazione**:
  - ✅ Testata installazione da zero su Fedora Linux
  - ✅ Testato recupero completo da backup
  - ⏳ Installazione Windows (previsto installer .exe)

### 2025-12-11 (sessione 12)
- Alert Guasti e AP Fault nel Dettaglio Permesso
- Fix visualizzazione dati nave/compagnia nel dettaglio permesso
- Miglioramento API AP Fault

### 2025-12-08 (sessioni 6-11)
- Supporto Telnet per Switch
- Auto-hostname Switch
- Sezione Uffici con gestione rete
- Sezione Sale Server con gestione rete
- Segnalazione Guasti Nave completa
- Navi raggruppate per compagnia in accordion
- Ampliamento anagrafica Compagnie con logo

### 2025-12-07 (sessioni 1-5)
- Sistema Calendario Trasferte + Note Spese
- Stampa/PDF foglio trasferte e nota spese
- Fix gestione rete nave (scan AP, MAC table)

### 2025-12-06
- Richiesta permessi porto completa
- Datepicker italiano globale

### 2025-12-03-05
- Setup iniziale progetto
- Anagrafiche base
- Magazzino e movimenti
- Autenticazione e ruoli

---

## TODO Futuro (Miglioramenti Opzionali)

### Priorità Alta
- [ ] **Installer Windows (.exe)** - Per semplificare installazione su Windows

### Miglioramenti UI/UX
- [ ] Dashboard con statistiche e grafici (chart.js)
- [ ] Icone nelle voci menu e bottoni
- [ ] Notifiche toast animate invece di alert
- [ ] Breadcrumb per navigazione
- [ ] Dark mode
- [ ] Animazioni sottili (transizioni, hover effects)
- [ ] Skeleton loading per caricamenti

### Funzionalità Aggiuntive
- [ ] Export Excel report (xlsx)
- [ ] Notifiche email automatiche (scadenze, promemoria)
- [ ] PWA per uso offline/mobile
- [ ] Log attività utenti (audit trail)
- [ ] Sistema notifiche interne
- [ ] Integrazione completa Observium

### Sicurezza
- [ ] Two-factor authentication (2FA)
- [ ] Rate limiting login
- [ ] Audit log accessi

### Performance
- [ ] Caching query frequenti
- [ ] Lazy loading immagini
- [ ] Compressione response gzip

---

## Note Installazione

### Linux (Fedora/RHEL)
```bash
# Installa Go
sudo dnf install golang

# Clona e compila
git clone https://github.com/furnaropolo/furviogest.git
cd furviogest
go build -o furviogest ./cmd/server

# Avvia
./furviogest -port 8080
```

### Ripristino da Backup
1. Avvia FurvioGest
2. Vai su /backup
3. Clicca "Carica Backup"
4. Seleziona file .zip
5. Conferma ripristino
6. Il server si riavvierà automaticamente

