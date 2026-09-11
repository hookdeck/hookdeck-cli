# Hookdeck Event Gateway Evals

These JSONL cases define expected behavior for the Hookdeck Event Gateway plugin
Skill. They are intentionally workflow-focused: the agent should inspect first,
ask for approval before mutation, and verify outcomes without exposing webhook
payloads or credentials.

Validate locally with:

```bash
while IFS= read -r line; do
  printf '%s\n' "$line" | jq -e . >/dev/null
done < plugins/hookdeck-event-gateway/evals/hookdeck-event-gateway/cases.jsonl
```
