# PaniniClaw - Because PocketClaw Was Already Taken
A claw app built on PocketBase.

## Installing Go

Because the developers of Go couldn't be bothered to write a half decent installer you can install Go to all users by running (after already having downloaded and placed the Go binary):
```
echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee /etc/profile.d/add_go_path.sh
sudo chmod +x /etc/profile.d/add_go_path.sh
```

## Installation

Run this script to install on Debian
```
su -
curl -fsSL https://raw.githubusercontent.com/impure/paniniclaw/main/setup.sh | bash
```
