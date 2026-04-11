#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root: sudo $0"
  exit 1
fi

if ! command -v apt-get >/dev/null 2>&1; then
  echo "This script currently supports Debian/Ubuntu systems with apt-get."
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required before configuring nvidia-container-toolkit."
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  apt-get update
  apt-get install -y curl
fi

if ! command -v gpg >/dev/null 2>&1; then
  apt-get update
  apt-get install -y gpg
fi

install -d -m 0755 /usr/share/keyrings

curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey \
  | gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg

curl -fsSL https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list \
  | sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' \
  > /etc/apt/sources.list.d/nvidia-container-toolkit.list

apt-get update
apt-get install -y nvidia-container-toolkit

nvidia-ctk runtime configure --runtime=docker
systemctl restart docker

echo
echo "nvidia-container-toolkit installed and Docker runtime configured."
echo "Validate with:"
echo "  docker info --format '{{json .Runtimes}}'"
echo "  docker run --rm --gpus all nvidia/cuda:12.6.3-base-ubuntu22.04 nvidia-smi"
