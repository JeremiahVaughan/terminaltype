#!/bin/bash
set -e
sudo cp "${HOME}/deploy/${APP}/${APP}.service" "/etc/systemd/system/${APP}.service"
sudo systemctl enable "${APP}.service"
sudo systemctl start "${APP}.service"
sudo systemctl restart "${APP}.service"
