#!/usr/bin/env bash
set -euo pipefail

cd /root/ecommerce_ai
. ./shared/main.env
echo "${DB_USER}:${DB_PASS}@tcp(${DB_HOST}:${DB_PORT})/jst_erp_r3_test?parseTime=true&multiStatements=true"
