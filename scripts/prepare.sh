#!/usr/bin/env bash
set -euo pipefail

echo "[prepare] installing deps (zip, tar, unzip, psql)..."

sudo apt-get update -y
sudo apt-get install -y --no-install-recommends \
  zip \
  tar \
  unzip \
  postgresql-client \
  curl

echo "[prepare] OK"
