#!/bin/sh
set -eu
mode=${1:-migrate}
case "$mode" in migrate|check) ;; *) exit 2 ;; esac
for scope in sessions catalog discount-reviews costs stock-settings replenishment-proposals suppliers supplier-offers purchase-drafts purchase-documents; do
  if [ "$mode" = migrate ]; then titan-online "migrate-$scope"; fi
  titan-online "check-$scope"
done
printf 'Esquemas online verificados no ambiente isolado.\n'
