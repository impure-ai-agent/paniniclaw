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

It is recommended to put a spend limit on your OpenRouter API key.

## Debugging

You can run as paniniclaw by doing:
```
sudo -u paniniclaw /bin/bash
```

## Github

- Install the Github CLI (gh). This allows you to create PRs using `gh pr create`.
- You will also need to create a classic token (not fine grained) with all repo permissions and `read:org`.
- Authenticate with gh using `echo "YOUR_GITHUB_TOKEN_HERE" | gh auth login --with-token`
- Create a `~/.netrc` file (or update it) with the following content:
```
machine api.github.com
    login <YOUR_GITHUB_USERNAME>
    password <YOUR_GITHUB_TOKEN>
```
- Finally add your Github bot account to be a contributor to your repo

## Commands

- **/stop** - Cancel the current response and stop the bot from thinking. Useful if you sent a message by accident or changed your mind.
- **/restart** - Restarts the service. Note that this does not rebuild it.

# Scheduled Tasks

Drop `.json` files here with a `"schedule"` key using cron syntax.

Example:

```json
{
  "schedule": "11 5 * * *",
  "name": "Wikipedia Rabbit Hole",
  "description": "Fetches a random Wikipedia article and summarizes it",
  "task": "Your task is to visit https://en.wikipedia.org/wiki/Special:Random using clean_curl.py, read the content, and tell the user what the article is about and share something interesting from it. When you are done, call the end_task tool."
}
```
