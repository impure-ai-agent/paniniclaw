#!/usr/bin/env bash
set -euo pipefail

# Check if running on Linux
if [ "$(uname)" != "Linux" ]; then
    echo "Warning: PaniniClaw is designed for Linux only. You are running on $(uname)." >&2
    echo "The setup may fail or behave unexpectedly." >&2
fi

# Check if the script is run as root
if [ "$(id -u)" -ne 0 ]; then
    echo "Error: This script must be run with sudo or as root to change file ownership." >&2
    exit 1
fi

USER_NAME="paniniclaw"
HOME_DIR="/home/${USER_NAME}"
APP_DIR="${HOME_DIR}/paniniclaw"
REPO_URL="https://github.com/impure/paniniclaw.git"
SERVICE_NAME="paniniclaw.service"

SECRETS_FILE="${APP_DIR}/secrets.json"

echo "==> Installing PaniniClaw"

# -----------------------------
# Create user if it doesn't exist
# -----------------------------
if ! id "$USER_NAME" >/dev/null 2>&1; then
    echo "==> Creating user $USER_NAME"

    useradd \
        --system \
        --create-home \
        --home-dir "$HOME_DIR" \
        --shell /usr/sbin/nologin \
        "$USER_NAME"

    passwd -l "$USER_NAME" >/dev/null 2>&1 || true
fi

# -----------------------------
# Clone or update repo
# -----------------------------
if [ ! -d "$APP_DIR/.git" ]; then
    echo "==> Cloning repo"
    sudo -u "$USER_NAME" git clone "$REPO_URL" "$APP_DIR"
else
    echo "==> Updating repo"
    sudo -u "$USER_NAME" git -C "$APP_DIR" pull
fi

# -----------------------------
# Build Go binary
# -----------------------------
echo "==> Building PaniniClaw"

sudo -u "$USER_NAME" bash -c "
    cd $APP_DIR
    /usr/local/go/bin/go mod download
    /usr/local/go/bin/go build -o paniniclaw .
"

chmod +x "$APP_DIR/paniniclaw"

# -----------------------------
# Ensure secrets.json exists
# -----------------------------
if [ ! -f "$SECRETS_FILE" ]; then
    echo "==> Creating secrets.json"

    cat > "$SECRETS_FILE" <<EOF
{
  "openrouter_api_key": "",
  "telegram_bot_token": ""
}
EOF
fi

# -----------------------------
# Permissions
# -----------------------------
chown "root:root" "$SECRETS_FILE"
chmod 600 "$SECRETS_FILE"

# find "$APP_DIR" -type f -name "*.go" -exec chown root:root {} +
# find "$APP_DIR" -type f -name "*.go" -exec chmod 644 {} +

# -----------------------------
# Install systemd service
# -----------------------------
echo "==> Installing systemd service"

cp "$APP_DIR/$SERVICE_NAME" \
   "/etc/systemd/system/$SERVICE_NAME"

chmod 644 "/etc/systemd/system/$SERVICE_NAME"

systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl restart "$SERVICE_NAME"

# -----------------------------
# Done
# -----------------------------
echo "==> PaniniClaw installed successfully"

systemctl --no-pager --full status "$SERVICE_NAME"