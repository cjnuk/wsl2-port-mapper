# Issue: Decode UTF-16 netsh output before parsing

## Summary
The service invokes `netsh` to reconcile Windows portproxy and firewall state. However, `netsh` emits UTF-16 text. The current code in `checkFirewallRules` and `getCurrentPortMappings` assumes UTF-8 and passes raw `cmd.Output()` bytes directly into string parsing helpers. On Windows, this fails to match existing rules and entries, causing warnings about missing permissions and repeated attempts to recreate state that already exists.

## Impact
- Validator reports false negatives for existing port proxy and firewall rules.
- Runtime reconciliation loops reapply mappings unnecessarily, wasting time and obscuring real errors.
- Firewall automation cannot distinguish between genuine failures and encoding issues, reducing operator confidence before release.

## Proposal
1. Add a helper (e.g., `decodeCommandOutput([]byte) (string, error)`) in `main.go` that detects UTF-16LE/BE command output via BOM or interleaved NUL bytes and converts it to UTF-8.
2. Use the helper to preprocess `netsh` command output in both `checkFirewallRules` and `getCurrentPortMappings`.
3. Extend `main_test.go` with tests that cover representative UTF-16LE samples to confirm decoding and parsing succeed.

## Testing
- To be covered by the proposed unit tests once implemented.
