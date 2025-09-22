# Issue: Strip UTF-16 BOM from WSL instance listings

## Summary
`getRunningWSLInstances` shells out to `wsl --list --running --quiet` and then splits the command output into distro names. On Windows, the command emits UTF-16 with a byte-order mark (BOM). After decoding to UTF-8, the code retains the leading `\ufeff` rune on the first line, so the first running instance never matches the configuration keys.

## Impact
- The first running WSL distribution is skipped during reconciliation, so any port mappings for that instance are never applied.
- Validators that check for running instances may report false negatives, reducing confidence in the service status.
- Operators may see flaky behavior where the first configured distro is ignored until another distribution starts and shifts the order.

## Proposal
1. Extend the command output decoding helper (or add a post-processing step) to detect and remove a leading BOM before splitting into lines.
2. Update `getRunningWSLInstances` to strip the BOM and ensure the resulting map keys match the expected distro names exactly.
3. Add regression tests to `main_test.go` that feed UTF-16LE samples containing a BOM and verify the decoded instance list is clean.

## Testing
- Covered by the proposed regression tests once implemented.
