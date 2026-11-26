#!/usr/bin/env bash
# Creates the local CA and server certificate. Run once per machine.
set -euo pipefail
cd "$(dirname "$0")"

# Git Bash would otherwise rewrite "/CN=..." into a Windows path.
export MSYS_NO_PATHCONV=1

if [[ -f ca.key && -f ca.crt ]]; then
  echo "Reusing existing CA (ca.key / ca.crt)."
else
  echo "Creating local CA..."
  openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes \
    -keyout ca.key -out ca.crt \
    -subj "/CN=Stashr Local Dev CA" \
    -addext "basicConstraints=critical,CA:TRUE" \
    -addext "keyUsage=critical,keyCertSign,cRLSign"
fi

echo "Creating server key + certificate signing request..."
openssl req -newkey rsa:2048 -nodes \
  -keyout server.key -out server.csr \
  -subj "/CN=localhost"

cat > server.ext <<'EOF'
basicConstraints=CA:FALSE
keyUsage=critical,digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth
subjectAltName=DNS:localhost,DNS:api-gateway,DNS:frontend,IP:127.0.0.1
EOF

echo "Signing server certificate with the CA..."
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out server.crt -days 825 -sha256 -extfile server.ext

rm -f server.csr server.ext

# The containers run as non-root users in some images; make the key readable.
chmod 644 server.key server.crt ca.crt

echo
echo "Done:"
ls -1 ca.crt server.crt server.key
echo
echo "Next: trust certs/ca.crt (see certs/README.md) so browsers show a padlock."
