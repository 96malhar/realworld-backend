#!/bin/bash

cleanup() {
  # Stop the web server if it is still running
  kill "$PID"

  # Clean up the binary
  rm -rf ./bin

  if [[ $SUCCESS == "true" ]]; then
      echo "====Smoke test passed===="
    fi
}

# Build the Go binary
go build -o ./bin/api ./cmd/api

# Get the version of the binary
VERSION=$(./bin/api -version | grep -o 'v\S*')

# Start the web server in the background
./bin/api &

# Capture the PID of the server process so we can kill it later
PID=$!

# Call the cleanup function on exit
trap cleanup EXIT ERR

# Wait for the server to start
sleep 2

# Make a GET request to the server and check the response
RESPONSE=$(curl http://localhost:4000/healthcheck)

SUCCESS="false"

# Check if the response contains the expected string
EXPECTED="{\"status\":\"available\",\"system_info\":{\"environment\":\"development\",\"version\":\"$VERSION\"}}"
if [[ $RESPONSE != "$EXPECTED" ]]; then
  echo "Smoke test failed. Server did not respond as expected."
  echo "Expected: $EXPECTED"
  echo "Actual  : $RESPONSE"
  exit 1
fi

# Register a new user
REGISTER_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:4000/users -H "Content-Type: application/json" -d '{"user":{"username":"smoketestuser","email":"smoketestuser@example.com","password":"smoketestpass"}}')
if [[ $REGISTER_STATUS -ne 201 ]]; then
  echo "Smoke test failed. User registration did not succeed. Status: $REGISTER_STATUS"
  exit 1
fi

# Login with the new user
LOGIN_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:4000/users/login -H "Content-Type: application/json" -d '{"user":{"email":"smoketestuser@example.com","password":"smoketestpass"}}')
if [[ $LOGIN_STATUS -ne 200 ]]; then
  echo "Smoke test failed. User login did not succeed. Status: $LOGIN_STATUS"
  exit 1
fi

SUCCESS="true"
exit 0
