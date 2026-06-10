# PaniniClaw
An lightweight AI assistant that leverages linux user accounts for security and openrouter for routing.

## Quick Start

First install Go and Git.

Because the developers of Go couldn't be bothered to write an installer you can install Go to all users by running (after already having downloaded and placed the Go binary):
```
echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee /etc/profile.d/add_go_path.sh
sudo chmod +x /etc/profile.d/add_go_path.sh
```

Although this will only run for root if you use `su -`. Then run this script to install PaniniClaw on Debian/Ubuntu. Note: it will create a new user called paniniclaw.
```
su -
curl -fsSL https://raw.githubusercontent.com/impure/paniniclaw/main/setup.sh | bash
```

You will then have to populate the secrets.json file with your API keys and restart the service.
```
sudo systemctl restart paniniclaw
```
