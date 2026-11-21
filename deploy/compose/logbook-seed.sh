#!/bin/sh
# Makes the logbook API key airlock calls logbook with, once, and keeps it in /keys for
# airlock to read. logbook shows a new key only when it makes it, so the key is saved then.
set -eu

key=/keys/logbook-airlock.key
if [ -s "$key" ]; then
  echo "logbook-seed: the key for airlock is there already"
  exit 0
fi

logbook migrate --config /etc/logbook/logbook.yaml
logbook key create airlock --json --config /etc/logbook/logbook.yaml |
  grep -o 'lbk_[A-Za-z0-9_-]*' >"$key.tmp"
[ -s "$key.tmp" ] || { echo "logbook-seed: no key in the output of logbook key create" >&2; exit 1; }
mv "$key.tmp" "$key"
echo "logbook-seed: made the key for airlock"
