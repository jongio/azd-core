# azd-app Integration Summary

**Quick Reference for Task 12 Analysis**

## TL;DR

✅ **Proceed with fileutil integration**  
❌ **No integration needed**: procutil, browser, shellutil  
📋 **Document for future**: pathutil enhancement  

---

## Integration Status by Package

| Package   | Status                  | Files  | Impact                                    |
|-----------|-------------------------|--------|-------------------------------------------|
| security  | ✅ Already integrated   | 21     | Path validation across codebase           |
| fileutil  | ⭐ High priority        | 4      | Fix critical bug + improve reliability    |
| pathutil  | 📋 Low priority         | 0      | Optional UX enhancement                   |
| procutil  | ❌ Not needed           | 0      | Container-based, no PID checks            |
| browser   | ❌ Not implemented      | 0      | Uses VS Code Simple Browser               |
| shellutil | ❌ Not needed           | 0      | Delegated to azd-exec extension           |

---

## fileutil Integration Plan

### Files to Modify (4 total)

1. **c:\code\azd-app\cli\src\internal\cache\reqs_cache.go**
   - Replace lines 259-276 with `fileutil.AtomicWriteJSON`
   - Impact: -12 lines, add retry logic + sync/flush

2. **c:\code\azd-app\cli\src\internal\config\notifications.go**
   - Replace lines 176-192 with `fileutil.AtomicWriteJSON`
   - Impact: -11 lines, improve reliability

3. **c:\code\azd-app\cli\src\internal\config\config.go**
   - Replace lines 86-92 with `fileutil.AtomicWriteJSON`
   - **CRITICAL**: Fixes non-atomic write bug
   - Impact: -4 lines, prevent config corruption

4. **c:\code\azd-app\cli\src\internal\service\detector.go**
   - Replace `containsText` with `fileutil.ContainsText`
   - Replace `fileExists` with `fileutil.FileExists`
   - Impact: Add path validation security

### Net Impact
- **Code reduction**: -50 lines
- **Reliability**: Add atomic writes, retry logic, sync/flush
- **Security**: Add path validation to detector helpers
- **Risk**: Low (drop-in API replacements)

---

## Implementation Checklist

### Prerequisites
- [ ] azd-core v0.2.0 published
- [ ] Update azd-app go.mod to require azd-core v0.2.0

### Implementation
- [ ] Update cache/reqs_cache.go (SaveResults method)
- [ ] Update config/notifications.go (Save method)
- [ ] Update config/config.go (Save method) - **CRITICAL FIX**
- [ ] Update service/detector.go (containsText, fileExists)

### Testing
- [ ] Run baseline tests: `go test ./...`
- [ ] Test cache persistence under crashes
- [ ] Test config integrity under crashes
- [ ] Test notification preferences save/load
- [ ] Verify .tmp file cleanup on failures

### Documentation
- [ ] Update CHANGELOG.md
- [ ] Document fileutil integration benefits
- [ ] Note critical config.go bug fix

---

## Benefits

### Code Quality
- Remove ~50 lines of duplicate temp file handling
- Consistent atomic write pattern
- Leverage battle-tested code (77.7% coverage)

### Reliability
- **Fix critical bug**: config.go non-atomic writes
- Add sync/flush before rename (CI-tested)
- Add retry logic (5 attempts with backoff)

### Security
- Add path validation to detector.go helpers
- Use security.ValidatePath before file reads

---

## Deferred Items

### pathutil (Low Priority)
- Enhanced error messages for missing tools
- Installation guidance with direct links
- Tool discovery in non-standard paths
- **Action**: Document for v0.3.0 UX improvements

### browser (Not Currently Needed)
- Dashboard uses VS Code Simple Browser
- Config exists but unused
- **Action**: Document for standalone mode (future)

### procutil (Not Needed)
- azd-app uses container orchestration, not PIDs
- No process.IsRunning() checks needed
- **Action**: None

### shellutil (Not Needed)
- azd-exec extension handles shell detection
- Clean separation of concerns
- **Action**: None

---

## Full Analysis

See [azd-app-integration-analysis.md](./azd-app-integration-analysis.md) for complete details.
