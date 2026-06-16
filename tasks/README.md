# Scheduled Tasks

Drop `.json` files here with a `"schedule"` key using cron syntax.

## Example

```json
{
  "schedule": "*/5 * * * *",
  "name": "News Summary",
  "description": "Summarize the news every 5 minutes",
  "system_prompt": "You are a news summarizer. Check the latest news and provide a brief summary."
}
```

The `system_prompt` field is optional. If provided, the scheduler will send it as a system prompt to the LLM on the scheduled interval.
