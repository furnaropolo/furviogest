#!/bin/bash
# FurvioGest - Script backup automatico
# Eseguito da cron ogni giorno a mezzanotte
# Chiama l'API interna per eseguire il backup

LOG_FILE="/var/log/furviogest_backup.log"
API_URL="http://127.0.0.1:8080/api/backup/automatico"

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Avvio backup automatico" >> "$LOG_FILE"

# Chiamata API
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL" 2>&1)
HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | head -n -1)

if [ "$HTTP_CODE" = "200" ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Backup completato con successo" >> "$LOG_FILE"
else
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERRORE backup - HTTP $HTTP_CODE: $BODY" >> "$LOG_FILE"
fi
