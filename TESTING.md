# Testing Markov Chain Feedback in ffuf

This repository includes tools to test the Markov Chain feedback functionality in ffuf.

## Setup

1. **Start the test server:**
   ```bash
   cd /home/fbogoslavskii/ffuf
   go run test/test_server.go
   ```

2. **In another terminal, run the test:**
   ```bash
   ./test_markov.sh
   ```

## Manual Testing

Alternatively, you can run tests manually:

```bash
# Basic test
ffuf -w test_wordlist.txt -u http://localhost:8080/FUZZ -mc 200,403 -t 50

# Verbose test to see the process
ffuf -w test_wordlist.txt -u http://localhost:8080/FUZZ -mc 200,403 -t 50 -v

# Test with different match codes
ffuf -w test_wordlist.txt -u http://localhost:8080/FUZZ -mc all -fc 404
```

## Test Server Endpoints

The test server has the following predictable responses:

### 200 OK Responses:
- `/admin` - Admin panel page
- `/login` - Login form
- `/user` - User profile
- `/api` - API endpoint
- `/config` - Configuration file
- `/settings` - Settings page
- `/dashboard` - Dashboard

### 403 Forbidden Responses:
- `/forbidden` - Forbidden access
- `/private` - Private area

### 404 Not Found:
- All other paths

## What to Expect

With Markov Chain feedback enabled (which is default), you should observe:

1. **Pattern Recognition**: The system learns which types of paths lead to 200 responses
2. **Adaptive Selection**: Inputs that previously led to matches are prioritized
3. **Response Analysis**: Different response characteristics (status codes, content length, etc.) influence future selections

## Verification

To verify the Markov Chain is working:

1. Run the same wordlist multiple times against the same target
2. Compare the input selection patterns between runs
3. Observe how the system adapts based on response patterns
4. Notice if the system learns to prefer paths that return 200 over those that return 404

The Markov Chain feedback operates transparently during normal ffuf operation, continuously learning and adapting the fuzzing strategy based on response patterns.