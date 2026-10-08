#!/usr/bin/env bash
# One-time setup for a fresh Ubuntu 24.04 EC2 instance.
#   curl -fsSL https://raw.githubusercontent.com/<you>/elcompresso/main/deploy/ec2-setup.sh | bash
# or, after cloning: bash deploy/ec2-setup.sh
set -euo pipefail

REPO_URL="${REPO_URL:-https://github.com/umohsamuel/elcompresso.git}"
APP_DIR="${APP_DIR:-$HOME/elcompresso}"

echo "==> Installing Docker"
if ! command -v docker >/dev/null; then
  curl -fsSL https://get.docker.com | sudo sh
  sudo usermod -aG docker "$USER"
fi

echo "==> Adding 2G swap (helps small instances during big encodes)"
if ! swapon --show | grep -q /swapfile; then
  sudo fallocate -l 2G /swapfile
  sudo chmod 600 /swapfile
  sudo mkswap /swapfile
  sudo swapon /swapfile
  echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab >/dev/null
fi

echo "==> Fetching code"
if [ ! -d "$APP_DIR/.git" ]; then
  git clone "$REPO_URL" "$APP_DIR"
else
  git -C "$APP_DIR" pull --ff-only
fi

cd "$APP_DIR/deploy"
if [ ! -f .env ]; then
  cp .env.example .env
  echo
  echo "Created deploy/.env - edit API_DOMAIN and ALLOWED_ORIGINS, then run:"
  echo "  cd $APP_DIR/deploy && sudo docker compose --profile caddy up -d --build"
  exit 0
fi

sudo docker compose --profile caddy up -d --build
sudo docker compose ps
echo "==> Done. Check: curl https://\$(grep ^API_DOMAIN .env | cut -d= -f2)/health"
