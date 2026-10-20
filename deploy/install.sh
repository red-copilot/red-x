#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID} -ne 0 ]]; then
  echo "run as root" >&2
  exit 1
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PREFIX="${PREFIX:-/opt/arl}"
CONFIG_DIR="${CONFIG_DIR:-/etc/arl}"
DATA_DIR="${DATA_DIR:-/var/lib/arl}"

id arl >/dev/null 2>&1 || useradd --system --home-dir "${PREFIX}" --shell /usr/sbin/nologin arl

install -d -m 0755 "${PREFIX}" "${PREFIX}/assets" "${PREFIX}/geoip" "${PREFIX}/secrets" "${CONFIG_DIR}" "${DATA_DIR}"
install -m 0755 "${ROOT}/arl" "${PREFIX}/arl"
cp -a "${ROOT}/assets/." "${PREFIX}/assets/"
if [[ ! -f "${CONFIG_DIR}/config.yaml" ]]; then
  install -m 0640 -o root -g arl "${ROOT}/config.example.yaml" "${CONFIG_DIR}/config.yaml"
  echo "created ${CONFIG_DIR}/config.yaml; edit secrets before starting"
fi
if [[ ! -f "${CONFIG_DIR}/arl.env" ]]; then
  install -m 0640 -o root -g arl "${ROOT}/deploy/arl.env.example" "${CONFIG_DIR}/arl.env"
  echo "created ${CONFIG_DIR}/arl.env; add environment overrides as needed"
fi
chown -R arl:arl "${DATA_DIR}"
chown -R root:arl "${PREFIX}/assets" "${PREFIX}/geoip" "${PREFIX}/secrets"
find "${PREFIX}/assets" "${PREFIX}/geoip" "${PREFIX}/secrets" -type d -exec chmod 0750 {} +
find "${PREFIX}/assets" "${PREFIX}/geoip" "${PREFIX}/secrets" -type f -exec chmod 0640 {} +

install -m 0644 "${ROOT}/deploy/systemd/arl.service" /etc/systemd/system/arl.service
systemctl daemon-reload

echo "installed ARL to ${PREFIX}"
echo "1. edit ${CONFIG_DIR}/config.yaml and ${CONFIG_DIR}/arl.env"
echo "2. run: ${PREFIX}/arl migrate --config ${CONFIG_DIR}/config.yaml"
echo "3. run: systemctl enable --now arl"
