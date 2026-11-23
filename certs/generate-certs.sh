#!/usr/bin/env bash
# Generates the TLS material for local HTTPS (run from Git Bash or any Unix shell):
#
#   ca.key / ca.crt          - a private "Stashr Local Dev CA" (trust ca.crt in your OS / Postman)
#   server.key / server.crt  - the server certificate used by BOTH nginx (frontend)
#                              and the api-gateway, signed by the CA above.
#
# The server certificate's Subject Alternative Names cover every hostname it is
# reached by:  localhost, 127.0.0.1 (browser / Postman), api-gateway (nginx -> gateway
# inside Docker) and frontend.
#
# Private keys are git-ignored; every developer generates their own.
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
