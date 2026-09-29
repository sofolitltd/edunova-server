#!/bin/sh
set -e

if [ -n "$FCM_SERVICE_ACCOUNT_JSON" ]; then
  echo "$FCM_SERVICE_ACCOUNT_JSON" > /app/serviceAccountKey.json
fi

exec ./server
