set -e

DEST=$1
APP=terminaltype

DEPLOY_PATH="${HOME}/deploy/${APP}"

if [ "$DEST" = "remote" ]; then            
    GOOS=linux GOARCH=arm64 go build -o ./deploy/app
    REMOTE_HOST="deploy.target"                                                     
    REMOTE_PATH="${REMOTE_HOST}:${DEPLOY_PATH}"                     
    rsync -avz --delete -e ssh ./deploy/ ./ui ./migrate "$REMOTE_PATH"
    ssh "${REMOTE_HOST}"  "${DEPLOY_PATH}/remote-deploy.sh"
else                                                                  
    GOOS=linux GOARCH=amd64 go build -o ./deploy/app
    LOCAL_PATH="${DEPLOY_PATH}"                               
    rsync -avz --delete ./deploy/ ./ui ./migrate "$LOCAL_PATH"
    "${DEPLOY_PATH}/remote-deploy.sh"
fi                                                                    




