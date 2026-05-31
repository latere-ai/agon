# Deploy Log

`make deploy` appends one row per agon-web prod rollout. Commit after each
ship so the audit trail lives in git. CLI binary releases live on the GitHub
releases page (driven by `release.yml` goreleaser).

| timestamp | version | rollout-output sha |
| --- | --- | --- |
