# FurvioGest - Sistema Gestionale Interventi Manutenzione WiFi/GSM

## Accesso
- **URL**: http://192.168.1.39:8080
- **Credenziali default**: admin / admin

---

## Funzionalità Implementate

### Anagrafiche
- [x] Tecnici (con upload documento identità, configurazione SMTP personale)
- [x] Fornitori
- [x] Porti (con dati agenzia: nome, email, telefono)
- [x] Automezzi (con upload libretto)
- [x] Compagnie (con opzione destinatari email: solo agenzia / tutti)
- [x] Navi (con email master, direttore macchina, ispettore + config Observium)

### Magazzino
- [x] Prodotti con gestione giacenza
- [x] Movimenti carico/scarico
- [x] Attrezzi con tracciamento posizione (sede/tecnico/nave)

### Apparati Nave
- [x] Gestione apparati per nave (router, switch, AP, firewall, ecc.)
- [x] Configurazione credenziali SSH/HTTP per ogni apparato
- [x] Integrazione predisposta per Observium

### Richiesta Permessi Porto
- [x] Creazione richiesta con: nave, porto, tecnici, automezzo, date
- [x] Tipo durata: giornaliera, multigiorno, fino a fine lavori
- [x] Campo Rientro in giornata (checkbox) - uso interno
- [x] Descrizione intervento (inclusa nell'email)
- [x] Anteprima email prima dell'invio
- [x] Invio email automatico all'agenzia (e opzionalmente a master/DDM/ispettore per Grimaldi)
- [x] Allegati automatici: documenti tecnici + libretto automezzo

### Calendario Trasferte + Note Spese Integrato
- [x] **Calendario mensile a griglia** (visualizzazione tipo foglio presenze)
- [x] **Colori per tipo giornata**:
  - Bianco = Ufficio
  - Giallo = Trasferta Giornaliera
  - Verde = Trasferta con Pernotto
  - Rosso = Trasferta Festiva
  - Blu = Ferie
  - Viola = Permesso
- [x] **Calcolo automatico festivi** (nazionali + Sant'Ambrogio per Milano)
- [x] **Click su giorno** apre modale per:
  - Selezionare tipo giornata
  - Se trasferta: inserire luogo, compagnia, nave
  - Aggiungere spese del giorno
- [x] **Tipi spesa**: Carburante, Cibo/Hotel, Pedaggi/Taxi, Materiali, Varie
- [x] **Metodo pagamento**: Carta aziendale / Carta personale (da rimborsare)
- [x] **Riepilogo mensile**: totale spese per categoria + totale da rimborsare
- [x] **Conteggio giorni** per tipo (ufficio, trasferte, ferie)
- [x] Scollegato dai permessi (sistema indipendente)
- [x] Editabile dal tecnico

### Stampa e Invio Email Documenti
- [x] **Anteprima Foglio Trasferte** (formato A4) con logo e dati azienda
- [x] **Anteprima Nota Spese** (formato A4) con logo e dati azienda
- [x] **Stampa / Salva PDF** diretta da anteprima (barra nascosta in stampa)
- [x] **Invio Email** da anteprima (richiede password per app Gmail)
- [x] **Campi email destinatari** in Impostazioni Azienda

### Rapporti Intervento
- [x] Creazione rapporti con dettaglio lavori
- [x] Collegamento a nave e tecnici

### DDT (Documenti di Trasporto)
- [x] Generazione DDT con righe prodotti
- [x] Numerazione automatica

### Foglio Mensile
- [x] Riepilogo mensile per tecnico

### Amministrazione
- [x] Dashboard separata per ruolo "amministrazione"
- [x] Visualizzazione rapporti, note spese, trasferte
- [x] Riepilogo mensile

### Impostazioni
- [x] Dati azienda (ragione sociale, indirizzo, P.IVA, ecc.)
- [x] Firma email personalizzabile (HTML)
- [x] Logo azienda
- [x] **Configurazione SMTP** per invio email
- [x] **Email destinatari** foglio trasferte e nota spese

### UI/UX
- [x] **Datepicker italiano** (formato GG/MM/AAAA) su tutti i campi data
- [x] Interfaccia responsive
- [x] Menu contestuale per ruolo (tecnico/amministrazione)

---

## Da Fare

### Priorità Alta
- [ ] **Rapporto di Intervento** - revisione completa interfaccia e funzionalità
- [ ] Invio email (richiede abilitazione "Password per le app" su Google Workspace)

### Priorità Media
- [ ] Export Excel riepilogo mensile
- [ ] Notifiche email per scadenze (permessi in scadenza, ecc.)
- [ ] Dashboard con statistiche

### Priorità Bassa
- [ ] Integrazione completa Observium (polling apparati)
- [ ] App mobile / PWA
- [ ] Backup automatico database
- [ ] Log attività utenti

---

## Da Testare

- [ ] Creazione/modifica/eliminazione anagrafiche (Tecnici, Fornitori, Porti, Automezzi, Compagnie, Navi)
- [ ] Magazzino: carico/scarico prodotti, movimenti attrezzi
- [ ] Permessi porto: creazione, anteprima email, invio (quando SMTP abilitato)
- [ ] Calendario trasferte: inserimento giornate, modifica, eliminazione
- [ ] Note spese: aggiunta spese, riepilogo corretto
- [ ] Stampa/PDF foglio trasferte e nota spese
- [ ] Rapporti intervento: creazione, modifica, stampa
- [ ] DDT: generazione, numerazione
- [ ] Impostazioni azienda: salvataggio dati, upload logo

---

## Note Tecniche

### Stack
- **Backend**: Go (Golang)
- **Database**: SQLite
- **Frontend**: HTML/CSS/JS + Go Templates
- **Datepicker**: Flatpickr (locale italiano)
- **Email**: SMTP (richiede Gmail con "Password per le app")

### File Principali Calendario Trasferte
- `internal/handlers/calendario_trasferte_handlers.go` - Handler principale calendario
- `internal/handlers/calendario_stampa_email.go` - Stampa e invio email
- `web/templates/calendario_trasferte.html` - Template calendario
- `web/templates/stampa_trasferte.html` - Template stampa foglio trasferte
- `web/templates/stampa_note_spese.html` - Template stampa nota spese

### Tabelle Database Calendario
- `calendario_giornate` - Dati giornalieri (tipo, luogo, nave, ecc.)
- `spese_giornaliere` - Spese collegate alle giornate

---

## Changelog

### 2025-12-07 (sessione 4)
- **Fix Nota Spese**:
  - Corretto parsing data (supporto formato RFC3339 da SQLite)
  - Ora la colonna Data mostra correttamente le date (es. 01/12)
- **Fix Logo nei documenti**:
  - Corretto percorso logo in Foglio Trasferte e Nota Spese
  - Ora usa la route /azienda/logo invece del path diretto

### 2025-12-07 (sessione 3)
- **Fix Calendario Trasferte**:
  - Corretti errori JavaScript (null check per elementi mancanti)
  - Ora il modale mostra correttamente sezione Spese e pulsante Elimina
  - Funziona eliminazione giornate (ferie, trasferte, ecc.)
- **Dati azienda dinamici nei documenti**:
  - Foglio Trasferte e Nota Spese ora mostrano i dati dalle Impostazioni
  - Logo, ragione sociale, indirizzo, P.IVA, telefono, email
  - Rimossi dati hardcoded dai template

### 2025-12-07 (sessione 2)
- **Anteprima documenti con barra azioni**:
  - Foglio Trasferte e Nota Spese visualizzabili in anteprima A4
  - Pulsante Stampa (apre dialogo stampa browser)
  - Pulsante Invia Email (con conferma)
  - Pulsante Indietro (torna al calendario)
  - Barra azioni nascosta in fase di stampa
- **Campi email in Impostazioni Azienda**:
  - email_foglio_trasferte
  - email_nota_spese
- Semplificata UI: un pulsante "Anteprima" invece di stampa+email separati

### 2025-12-07 (sessione 1)
- **NUOVO Sistema Calendario Trasferte**:
  - Calendario mensile a griglia con colori per tipo giornata
  - Integrazione note spese direttamente nel giorno
  - Calcolo automatico festivi italiani + Sant'Ambrogio (Milano)
  - Riepilogo mensile con totali spese e da rimborsare
  - Sistema indipendente (scollegato dai permessi)

### 2025-12-06
- Aggiunto campo "Rientro in giornata" ai permessi
- Generazione automatica trasferte da permessi (quando pernotto)
- Collegamento trasferte <-> richieste permesso
- Sostituito "Km percorsi" con "Nave" nelle trasferte
- Fix salvataggio "Descrizione intervento" nei permessi
- Implementato datepicker italiano (Flatpickr) globale

### 2025-12-05
- Implementata funzionalità completa richiesta permessi
- Anteprima e invio email con allegati
- Gestione destinatari email per compagnia

### 2025-12-04
- Gestione apparati nave
- Configurazione Observium
- Miglioramenti UI vari

### 2025-12-03
- Setup iniziale progetto
- Anagrafiche base
- Autenticazione e ruoli

### 2025-12-07 (sessione 5)
- **Gestione Rete Nave - AC e Switch**:
  - Fix scan AP da Access Controller Huawei
  - Corretto parser output "display ap all" per formato reale Huawei
  - Mapping stato AP: nor → online, idle → offline
  - Fix scan tabella MAC da switch Huawei
  - Corretto parser output "display mac-address" con porte GE/XGE
  - Gestione paginazione "---- More ----" negli switch
  - Associazione automatica AP ↔ porta switch tramite MAC address
  - Supporto porte Huawei formato 0/0/x e 1/0/x
  - Backup automatico configurazione switch

### 2025-12-08 (sessione 6)
- **Supporto Telnet per Switch**:
  - Aggiunto campo "Protocollo" (SSH/Telnet) per ogni switch
  - Cambio automatico porta quando si seleziona protocollo (22→SSH, 23→Telnet)
  - Implementata funzione executeTelnetCommand per switch senza SSH
  - Test connessione supporta entrambi i protocolli
  - Backup configurazione via Telnet funzionante
- **Fix eliminazione AC**:
  - Eliminazione AC ora rimuove anche i file di backup associati
  - Pulizia corretta record config_backup per tipo 'ac'
- **Fix JavaScript rete_nave.html**:
  - Risolto errore sintassi JS che impediva caricamento funzioni
  - Corretto testSSHAC con gestione errori (.catch)
  - Rimosso codice duplicato che causava errori di parsing

### 2025-12-08 (sessione 7)
- **Auto-hostname Switch**:
  - Rimosso campo "Nome" dal form di inserimento switch
  - Il nome viene ora recuperato automaticamente dall'hostname dello switch
  - Funzione getSwitchHostname: esegue comando "display current-configuration | include sysname" (Huawei) o "show running-config | include hostname" (HP)
  - Supporta sia SSH che Telnet in base alla configurazione
  - Fallback: se non riesce a recuperare l'hostname, usa "Switch-IP"
  - In modifica switch: il nome esistente non viene modificato
- **Fix Test Connessione Telnet**:
  - APITestSSH ora supporta correttamente sia SSH che Telnet
  - Script expect separati per i due protocolli
  - Messaggi di errore specifici per protocollo
- **Fix Backup/Modello via Telnet**:
  - Backup configurazione funziona correttamente via Telnet
  - Recupero modello usa il protocollo corretto dello switch
- **Scan LLDP/MAC funzionante**:
  - Testato su Zeus Palace con 5 switch
  - Associazione automatica AP ↔ porta switch tramite MAC

### 2025-12-08 (sessione 8)
- **Nuova sezione Uffici**:
  - CRUD completo (nome, indirizzo, città, CAP, telefono, email, note)
  - Gestione rete con AC e Switch (solo backup, no scan)
  - Backup configurazione manuale e automatico settimanale
- **Nuova sezione Sale Server**:
  - CRUD completo (nome, indirizzo, città, CAP, telefono, email, note)
  - Gestione rete con soli Switch (solo backup, no scan)
  - Backup configurazione manuale e automatico settimanale
- **Tabelle DB aggiunte**:
  - uffici, ac_ufficio, switch_ufficio
  - sale_server, switch_sala_server
  - config_backup_ufficio
- **Integrazione backup settimanale**:
  - Job settimanale esteso per includere uffici e sale server
  - Supporto SSH e Telnet per tutti gli apparati

### 2025-12-08 (sessione 9)
- **Nuova sezione Segnalazione Guasti Nave**:
  - Lista navi con conteggio guasti attivi e badge colore per gravità
  - Pagina guasti per singola nave con card colorate (rosso=alta, giallo=media, grigio=bassa)
  - Ogni guasto ha: data apertura, gravità (bassa/media/alta), descrizione, stato (aperto/preso in carico/risolto)
  - Quando risolto: data risoluzione, tecnico che ha risolto, descrizione risoluzione
  - **Auto-inserimento AP fault**: quando uno scan AP rileva un AP in fault, viene creato automaticamente un guasto
  - **Auto-chiusura AP fault**: quando l'AP torna online, il guasto viene chiuso automaticamente con descrizione "AP tornato online"
  - **Storico guasti**: filtro per data inizio/fine, statistiche per gravità
- **Tabelle DB aggiunte**:
  - guasti_nave (id, nave_id, tipo, ap_id, gravita, descrizione, stato, tecnico_apertura_id, data_apertura, tecnico_risoluzione_id, data_risoluzione, descrizione_risoluzione)
- **Templates creati**:
  - guasti_navi_lista.html - lista navi con conteggio
  - guasti_nave.html - guasti singola nave con CRUD
  - guasti_storico.html - storico con filtro date
- **Routes aggiunte**:
  - /guasti-nave - lista navi
  - /guasti-nave/{id} - guasti nave
  - /guasti-nave/nuovo/{id} - nuovo guasto
  - /guasti-nave/modifica/{id} - modifica guasto
  - /guasti-nave/elimina/{id} - elimina guasto
  - /guasti-nave/storico - storico guasti

## TODO - Prossima sessione
1. **TEST Segnalazione Guasti Nave**:
   - Testare lista navi con conteggio guasti
   - Testare inserimento nuovo guasto manuale
   - Testare modifica stato guasto (aperto → preso in carico → risolto)
   - Testare storico guasti con filtro date
   - Testare auto-inserimento guasto quando AP va in fault (scan AP)
   - Testare auto-chiusura guasto quando AP torna online

2. **TEST Uffici e Sale Server** (se non già testato):
   - Testare CRUD uffici
   - Testare CRUD sale server
   - Testare backup configurazione switch/AC

### 2025-12-08 (sessione 10)
- **TEST Segnalazione Guasti Nave** - COMPLETATO:
  - ✅ Lista navi con conteggio guasti e badge colorati per gravità
  - ✅ Inserimento nuovo guasto manuale (form modale con gravità e descrizione)
  - ✅ Modifica stato guasto (aperto → preso in carico → risolto)
  - ✅ Storico guasti con filtro date (fix: allineati nomi parametri form data_da/data_a)
  - ✅ Auto-inserimento guasto quando AP in fault (testato con simulazione DB)
  - ✅ Auto-chiusura guasto quando AP torna online (descrizione automatica)
- **TEST Uffici e Sale Server** - COMPLETATO:
  - ✅ CRUD Uffici (creazione, modifica, lista)
  - ✅ Gestione rete Uffici (AC + Switch con backup)
  - ✅ CRUD Sale Server (creazione, modifica, lista)
  - ✅ Gestione rete Sale Server (solo Switch con backup)
- **Miglioramento Lista Navi**:
  - Navi ora raggruppate per compagnia in accordion collassabili
  - Spazio per logo compagnia in ogni header accordion
  - Pulsanti "Espandi tutti" / "Chiudi tutti"
  - Filtro ricerca nasconde automaticamente compagnie vuote
- **Ampliamento Anagrafica Compagnie**:
  - Nuovi campi sede legale: città, CAP, provincia, P.IVA, codice fiscale
  - Upload logo compagnia
  - Form riorganizzato in sezioni card (Dati, Sede Legale, Contatti, Logo)
  - Route /compagnie/logo/{id} per servire i loghi
- **Aggiornamenti DB**:
  - Tabella compagnie: aggiunti campi citta, cap, provincia, piva, codice_fiscale, logo

## TODO - Prossima sessione
1. Testare upload logo compagnie e visualizzazione in lista navi
2. Completare test backup configurazione su uffici/sale server reali
3. Revisione Rapporto di Intervento (priorità alta da backlog)

### 2025-12-08 (sessione 11)
- **Gestione Guasti Nave - Selezione Tecnico**:
  - Aggiunta selezione tecnico obbligatoria quando si cambia stato a 'preso_in_carico'
  - Aggiunta selezione tecnico obbligatoria quando si cambia stato a 'risolto'
  - Campo note per presa in carico (opzionale)
  - Campo descrizione risoluzione (obbligatorio)
  - Visualizzazione tecnico e note nella card del guasto
- **Fix sezione AP Fault**:
  - Cambiato da classe 'alert' a 'card' per evitare auto-hide JavaScript
  - La sezione AP in fault/offline ora rimane visibile permanentemente
- **Aggiornamenti DB**:
  - guasti_nave: aggiunti campi tecnico_presa_in_carico_id, data_presa_in_carico, note_presa_in_carico

## TODO - Prossima sessione
1. Testare se i fault AP vengono mostrati anche nella richiesta permessi
2. Revisione Rapporto di Intervento (priorità alta da backlog)
