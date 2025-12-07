#!/bin/bash
IP=$1
PORT=$2
USER=$3
PASS=$4
CMD=${5:-"display version"}

# Prima prova con sshpass
OUTPUT=$(sshpass -p "$PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 -o PreferredAuthentications=keyboard-interactive,password -o PubkeyAuthentication=no -p $PORT $USER@$IP "$CMD" 2>&1)
EXIT_CODE=$?

if [ $EXIT_CODE -eq 0 ]; then
    echo "SUCCESS"
    echo "$OUTPUT"
    exit 0
fi

# Se fallisce, prova con expect se disponibile
if command -v expect &> /dev/null; then
    OUTPUT=$(expect -c "
        set timeout 15
        spawn ssh -o StrictHostKeyChecking=no -p $PORT $USER@$IP $CMD
        expect {
            \"*assword*\" { send \"$PASS\r\"; exp_continue }
            \"*>\" { exit 0 }
            timeout { exit 1 }
            eof { exit 0 }
        }
    " 2>&1)
    if [ $? -eq 0 ]; then
        echo "SUCCESS"
        echo "$OUTPUT"
        exit 0
    fi
fi

echo "FAILED"
echo "$OUTPUT"
exit 1
