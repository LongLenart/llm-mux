#!/bin/bash

echo "Testing /v1/messages with Parallel Tool Calls..."

curl -s -X POST http://localhost:8318/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: test-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "gemini-claude-sonnet-4-5",
    "max_tokens": 1024,
    "tools": [
      {
        "name": "get_weather",
        "description": "Get current weather for a city",
        "input_schema": {
          "type": "object",
          "properties": {
            "city": { "type": "string", "description": "City name" }
          },
          "required": ["city"]
        }
      },
      {
        "name": "get_time",
        "description": "Get current time for a city",
        "input_schema": {
          "type": "object",
          "properties": {
            "city": { "type": "string", "description": "City name" }
          },
          "required": ["city"]
        }
      }
    ],
    "messages": [
      {
        "role": "user",
        "content": "Please check the weather and time for both Tokyo and New York simultaneously."
      }
    ]
  }' | jq .

echo ""
echo "Testing /v1/messages with Parallel Tool Calls (STREAMING)..."

curl -N -s -X POST http://localhost:8318/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: test-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "gemini-claude-sonnet-4-5",
    "max_tokens": 1024,
    "stream": true,
    "tools": [
      {
        "name": "get_weather",
        "description": "Get current weather for a city",
        "input_schema": {
          "type": "object",
          "properties": {
            "city": { "type": "string", "description": "City name" }
          },
          "required": ["city"]
        }
      },
      {
        "name": "get_time",
        "description": "Get current time for a city",
        "input_schema": {
          "type": "object",
          "properties": {
            "city": { "type": "string", "description": "City name" }
          },
          "required": ["city"]
        }
      }
    ],
    "messages": [
      {
        "role": "user",
        "content": "Please check the weather and time for both Tokyo and New York simultaneously. Use parallel tool execute."
      }
    ]
  }'
