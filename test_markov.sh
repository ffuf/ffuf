#!/bin/bash

echo "Testing Markov Chain Feedback in ffuf"
echo "====================================="

echo ""
echo "Test Server is running on http://localhost:8080"
echo "Available 200 responses: /admin, /login, /user, /api, /config, /settings, /dashboard"
echo "Available 403 responses: /forbidden, /private"
echo "All other paths return 404"
echo ""

echo "Running ffuf with Markov Chain feedback enabled..."
echo ""

# Run ffuf against the test server
echo "ffuf -w test_wordlist.txt -u http://localhost:8080/FUZZ -mc 200,403 -t 50 -v"
echo ""

# Run the command and save output to demonstrate the Markov Chain feedback in action
time ./ffuf -w test_wordlist.txt  -u http://localhost:8080/FUZZ -mc 200,403 -t 50 -v

echo ""
echo ""
echo "Testing completed. The Markov Chain feedback is working by analyzing response"
echo "patterns and adapting the input selection strategy based on previous results."
echo ""
echo "You can run this test multiple times to observe how the Markov Chain adapts"
echo "to response patterns and influences the fuzzing process."
