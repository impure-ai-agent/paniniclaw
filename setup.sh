#!/usr/bin/env bash
set -euo pipefail

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
# Ensure required keys exist
# (merge-style patch using jq if available, fallback otherwise)
# -----------------------------
if command -v jq >/dev/null 2>&1; then
    TMP=$(mktemp)

    jq '
      .openrouter_api_key = (.openrouter_api_key // "")
      | .telegram_bot_token = (.telegram_bot_token // "")
    ' "$SECRETS_FILE" > "$TMP"

    mv "$TMP" "$SECRETS_FILE"
else
    echo "==> jq not found, skipping merge step (install jq for best results)"
fi

# -----------------------------
# Permissions
# -----------------------------
chown -R "$USER_NAME:$USER_NAME" "$APP_DIR"

chmod 600 "$SECRETS_FILE"

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