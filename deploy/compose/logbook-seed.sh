#!/bin/sh
# Makes the logbook API key airlock calls logbook with, and keeps it in /keys for airlock to
# read. logbook shows a new key only when it makes it, so the key is saved then. A saved key
# that logbook no longer knows, such as after its volume was removed, is made again.
set -eu

cfg=/etc/logbook/logbook.yaml
key=/keys/logbook-airlock.key

logbook migrate --config "$cfg"

if [ -s "$key" ]; then
  last4=$(tr -d '\n' <"$key" | tail -c 4)
  if logbook key list --config "$cfg" | awk -v l="...$last4" '$2 == "airlock" && $3 == l { found = 1 } END { exit !found }'; then
    echo "logbook-seed: the key for airlock is there already"
    exit 0
  fi
fi

for id in $(logbook key list --config "$cfg" | awk '$2 == "airlock" { print $1 }'); do
  logbook key delete "$id" --config "$cfg"
done
logbook key create airlock --json --config "$cfg" | grep -o 'lbk_[A-Za-z0-9_-]*' >"$key.tmp"
[ -s "$key.tmp" ] || { echo "logbook-seed: no key in the output of logbook key create" >&2; exit 1; }
mv "$key.tmp" "$key"
echo "logbook-seed: made the key for airlock"
