#!/bin/sh
# Writes nginx's site config from nginx.conf.template, taking the limits from
# config.yaml so they are changed in one place. nginx cannot read YAML itself.
# The nginx image runs this (/docker-entrypoint.d/40-stashr-config.sh) before
# nginx starts; if it fails, the container does not start.
#
# Only simple top-level "key: value" lines are read. A missing key gets the same
# default as in the Go services.
set -eu

CONFIG="${CONFIG_PATH:-/etc/stashr/config.yaml}"
TEMPLATE="${NGINX_TEMPLATE:-/etc/nginx/stashr.conf.template}"
OUTPUT="${NGINX_OUTPUT:-/etc/nginx/conf.d/default.conf}"

fail() {
    echo "stashr config: $*" >&2
    exit 1
}

[ -r "$CONFIG" ] || fail "cannot read $CONFIG (is config.yaml mounted?)"

# value KEY DEFAULT
value() {
    found=$(sed -n "s/^$1:[[:space:]]*\([^#[:space:]]*\).*/\1/p" "$CONFIG" | tail -n 1 | tr -d "\"'")
    echo "${found:-$2}"
}

# positive_number KEY DEFAULT
positive_number() {
    result=$(value "$1" "$2")
    case "$result" in
        *[!0-9]* | 0*) fail "$1 must be a positive whole number, got '$result'" ;;
    esac
    echo "$result"
}

max_json=$(positive_number max_json_body_bytes 1048576)
max_upload=$(positive_number max_upload_bytes 209715200)
overhead=$(positive_number upload_multipart_overhead_bytes 1048576)
upload_timeout=$(value upload_timeout 10m)

# The forms nginx and Go both understand, e.g. 90s, 10m, 1h30m.
echo "$upload_timeout" | grep -Eq '^([0-9]+(h|m|s|ms))+$' ||
    fail "upload_timeout must use whole units, e.g. 90s, 10m or 1h30m, got '$upload_timeout'"

sed -e "s/__MAX_JSON_BODY_BYTES__/$max_json/g" \
    -e "s/__MAX_UPLOAD_REQUEST_BYTES__/$((max_upload + overhead + 1048576))/g" \
    -e "s/__UPLOAD_TIMEOUT__/$upload_timeout/g" \
    "$TEMPLATE" >"$OUTPUT"

echo "stashr config: wrote $OUTPUT from $CONFIG"
