#!/bin/bash

set -e


export APP=terminaltype
FILES=("./deploy/" "./ui" "./migrate")


DEST=$1
DEPLOY_PATH="${HOME}/deploy/${APP}"
if [ "$DEST" = "remote" ]; then            
    GOOS=linux GOARCH=arm64 go build -o "./deploy/${APP}"
    REMOTE_HOST="deploy.target"                                                     
    REMOTE_PATH="${REMOTE_HOST}:${DEPLOY_PATH}"                     
    rsync -avz --delete -e ssh "${FILES[@]}" "$REMOTE_PATH"          
    ssh "${REMOTE_HOST}" "APP=${APP} ${DEPLOY_PATH}/remote-deploy.sh"
else                                                                  
    GOOS=linux GOARCH=amd64 go build -o "./deploy/${APP}"
    LOCAL_PATH="${DEPLOY_PATH}"                               
    rsync -avz --delete "${FILES[@]}" "$LOCAL_PATH"
    "${DEPLOY_PATH}/remote-deploy.sh"
fi                                                                    




