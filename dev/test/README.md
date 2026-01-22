# Coders Test Suite

Development tests and testing documentation for the coders plugin.

## Test Files

- **`test-promises.sh`** - Automated test for promises feature
- **`TESTING-GUIDE.md`** - Comprehensive testing guide for promises
- **`TEST-PROMISES.md`** - Detailed test scenarios and cases
- **`coders.test.js`** - Unit tests for coders functionality
- **`test.js`** - General test utilities

## Running Tests

### Promises Feature Test

```bash
cd /Users/beepboop/dev/coders
./dev/test/test-promises.sh
```

### Manual Testing

See `TESTING-GUIDE.md` for detailed manual testing instructions.

## Test Coverage

- ✅ Promise publishing and retrieval
- ✅ CLI commands (`/coders:promises`)
- ✅ Dashboard integration
- ✅ Redis persistence
- ✅ Session spawning and completion
- ✅ Multiple status types (completed, blocked, needs-review)
