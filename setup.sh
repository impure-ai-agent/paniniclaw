#!/usr/bin/env bash
set -euo pipefail

USER_NAME="paniniclaw_agent"
HOME_DIR="/home/${USER_NAME}"
APP_DIR="${HOME_DIR}/paniniclaw"
REPO_URL="https://github.com/impure/PanoptiClaw.git"
SERVICE_NAME="paniniclaw.service"

if ! id "$USER_NAME" >/dev/null 2>&1; then
    useradd \
        --system \
        --create-home \
        --home-dir "$HOME_DIR" \
        --shell /usr/sbin/nologin \
        "$USER_NAME"

    passwd -l "$USER_NAME" >/dev/null 2>&1 || true
fi

if [ ! -d "$APP_DIR/.git" ]; then
    sudo -u "$USER_NAME" git clone \
        "$REPO_URL" \
        "$APP_DIR"
else
    sudo -u "$USER_NAME" git -C "$APP_DIR" pull
fi

cp \
    "$APP_DIR/paniniclaw.service" \
    "/etc/systemd/system/$SERVICE_NAME"

chmod 644 "/etc/systemd/system/$SERVICE_NAME"

systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl restart "$SERVICE_NAME"