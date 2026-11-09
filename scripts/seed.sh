#!/usr/bin/env bash
# KutubCore demo seed (Blueprint §19: seed data for a fresh install).
# Requires the server running on $ADDR (default :8080).
# Creates: organizer demo user, 2 writers, 2 books (one Bengali), 1 bookshop,
# 1 source, 1 acquisition (2 copies), 1 PDF upload, 1 library entry.
#
#   ./scripts/seed.sh
#   ADDR=http://localhost:8080 ./scripts/seed.sh
set -euo pipefail
BASE="${ADDR:-http://localhost:8080}"
API="$BASE/api/v1"
CJ="$(mktemp)"
trap 'rm -f "$CJ"' EXIT

echo "Registering demo organizer…"
curl -s -c "$CJ" -H 'Content-Type: application/json' \
  -d '{"display_name":"Demo Organizer","email":"demo@kutub.example","password":"demodemo123"}' \
  "$API/auth/register" > /dev/null
curl -s -b "$CJ" -H 'Content-Type: application/json' -d '{}' \
  "$API/users/me/roles/organizer" > /dev/null

W1=$(curl -s -b "$CJ" -H 'Content-Type: application/json' -d '{"full_name":"Rabindranath Tagore"}' "$API/writers" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['id'])")
W2=$(curl -s -b "$CJ" -H 'Content-Type: application/json' -d '{"full_name":"Humayun Ahmed"}' "$API/writers" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['id'])")
B1=$(curl -s -b "$CJ" -H 'Content-Type: application/json' -d "{\"title\":\"Gitanjali\",\"original_title\":\"গীতাঞ্জলি\",\"writers\":[{\"id\":\"$W1\",\"role\":\"author\"}]}" "$API/books" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['id'])")
B2=$(curl -s -b "$CJ" -H 'Content-Type: application/json' -d "{\"title\":\"Shonkhonil\",\"writers\":[{\"id\":\"$W2\",\"role\":\"author\"}]}" "$API/books" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['id'])")

# Bengali edition for Gitanjali.
LANG_BN=$(curl -s "$API/languages" | python3 -c "import sys,json;print(next(x['id'] for x in json.load(sys.stdin)['data'] if x['code']=='bn'))")
curl -s -b "$CJ" -H 'Content-Type: application/json' -d "{\"language_id\":\"$LANG_BN\",\"publisher\":\"Demo Pub\",\"publication_year\":1912,\"format\":\"paperback\",\"page_count\":120}" "$API/books/$B1/editions" > /dev/null

SHOP=$(curl -s -b "$CJ" -H 'Content-Type: application/json' -d '{"name":"Dhaka Book House","shop_type":"both","city":"Dhaka"}' "$API/bookshops" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['id'])")
curl -s -b "$CJ" -H 'Content-Type: application/json' -d "{\"items\":[{\"book_id\":\"$B1\",\"format\":\"paperback\",\"quantity\":2}]}" "$API/me/acquisitions" > /dev/null

PDF="$(mktemp --suffix=.pdf)"
printf '%%PDF-1.4 KutubCore seed\n' > "$PDF"
curl -s -b "$CJ" -F "file=@$PDF;filename=gitanjali.pdf" "$API/books/$B1/files" > /dev/null

curl -s -b "$CJ" -H 'Content-Type: application/json' -d "{\"book_id\":\"$B1\",\"physical_status\":\"owned\",\"digital_status\":\"downloaded\",\"reading_status\":\"currently_reading\",\"is_favorite\":true}" "$API/me/books" > /dev/null

echo "Seeded: writers=$W1,$W2 books=$B1,$B2 shop=$SHOP"
echo "Demo login: demo@kutub.example / demodemo123"
