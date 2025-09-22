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

## Resolution

**Status: ✅ RESOLVED**

**Implemented in commit**: e190944 - "fix: decode UTF-16 netsh output before parsing"

**Solution implemented:**
1. ✅ Added `decodeCommandOutput([]byte) (string, error)` helper function in `main.go`
2. ✅ Helper detects UTF-16LE via BOM (0xFF, 0xFE) or interleaved NUL bytes
3. ✅ Updated `checkFirewallRules()` to preprocess netsh firewall command output
4. ✅ Updated `getCurrentPortMappings()` to preprocess netsh portproxy command output
5. ✅ Refactored `getRunningWSLInstances()` to use the same helper (DRY principle)
6. ✅ Added comprehensive unit tests in `main_test.go`:
   - UTF-16LE with and without BOM
   - WSL instance names, netsh output headers, port numbers
   - Windows line endings, fallback scenarios
   - Edge cases (odd length bytes, non-UTF-16 patterns)

**Testing results:**
- ✅ All existing tests pass
- ✅ 10 new UTF-16 decoding tests added and passing
- ✅ Configuration validation now works correctly
- ✅ No more false negatives for existing firewall/portproxy rules

**Impact:**
- Eliminates false warnings about missing permissions
- Prevents unnecessary recreation of existing state
- Improves operator confidence and reduces noise in logs
- Provides robust UTF-16 handling for all Windows command output
