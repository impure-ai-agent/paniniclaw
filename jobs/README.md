# Scheduled Jobs

Drop `.json` files here with a `"schedule"` key using cron syntax.

## Example

```json
{
  "schedule": "*/5 * * * *",
  "description": "Does a thing every 5 minutes",
  "command": "echo 'hello'"
}
```

The `command` field is optional (placeholder for now - proof of concept).
