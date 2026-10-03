#!/usr/bin/env bash
#MISE description="Stop the test SMTP server (Mailpit)"
set -euo pipefail
docker compose -f docker-compose.test.yaml rm -sf mailpit
