#!/bin/sh
# Explicit operator-run DEVELOPMENT-only mTLS credential generator.
# Creates a NEW seven-day private CA, SoR server certificate and ingest client
# certificate. NEVER run this automatically from jail startup or CI packaging.
# Never copies keys, alters C220 jails/mounts/SQL, or touches live credentials.
set -eu
umask 077

if [ "$#" -ne 2 ] || [ "$1" != "--generate-dev-only" ]; then
 echo "usage: sh pathfinder_c1_dev_pki.sh --generate-dev-only /ABSOLUTE/NEW/PRIVATE/DIR" >&2
 exit 2
fi
dest=$2
case "$dest" in
 /*) ;;
 *) echo "PATHFINDER_C1_PKI_FAIL absolute destination directory required" >&2; exit 1 ;;
esac
case "$dest" in
 /|/etc|/usr|/var|/root|/tmp|*/../*|*/./*|*' '*) 
  echo "PATHFINDER_C1_PKI_FAIL unsafe destination" >&2;exit 1 ;;
esac
command -v openssl >/dev/null 2>&1 ||
 { echo "PATHFINDER_C1_PKI_FAIL OpenSSL required" >&2; exit 1; }
[ ! -e "$dest" ] && [ ! -L "$dest" ] ||
 { echo "PATHFINDER_C1_PKI_FAIL destination already exists; no overwrite" >&2; exit 1; }
mkdir -m 0700 "$dest" ||
 { echo "PATHFINDER_C1_PKI_FAIL unable to create new private directory" >&2;exit 1; }

openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out "$dest/ca.key"
openssl req -new -x509 -sha256 -days 7 \
  -key "$dest/ca.key" -out "$dest/ca.crt" \
  -subj "/CN=Pathfinder C1 Development Isolated CA" \
  -addext "basicConstraints=critical,CA:TRUE" \
  -addext "keyUsage=critical,keyCertSign,cRLSign"

for component in sor ingest; do
 case "$component" in
  sor) dns=pathfinder-sor.internal; usage=serverAuth ;;
  ingest) dns=pathfinder-ingest.internal; usage=clientAuth ;;
 esac
 openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 \
   -out "$dest/$component.key"
 openssl req -new -sha256 -key "$dest/$component.key" \
   -subj "/CN=$dns" -out "$dest/$component.csr"
 printf 'basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=%s\nsubjectAltName=DNS:%s\n' \
   "$usage" "$dns" > "$dest/$component.ext"
 openssl x509 -req -sha256 -days 7 \
   -in "$dest/$component.csr" -CA "$dest/ca.crt" \
   -CAkey "$dest/ca.key" -CAcreateserial \
   -extfile "$dest/$component.ext" -out "$dest/$component.crt"
done

openssl verify -purpose sslserver -CAfile "$dest/ca.crt" "$dest/sor.crt" >/dev/null
openssl verify -purpose sslclient -CAfile "$dest/ca.crt" "$dest/ingest.crt" >/dev/null
openssl x509 -in "$dest/sor.crt" -noout -ext subjectAltName |
    grep -Fq 'DNS:pathfinder-sor.internal' ||
 { echo "PATHFINDER_C1_PKI_FAIL missing SoR DNS SAN" >&2; exit 1; }
openssl x509 -in "$dest/ingest.crt" -noout -ext subjectAltName |
    grep -Fq 'DNS:pathfinder-ingest.internal' ||
 { echo "PATHFINDER_C1_PKI_FAIL missing ingest DNS SAN" >&2; exit 1; }

rm -f "$dest/sor.csr" "$dest/ingest.csr" \
 "$dest/sor.ext" "$dest/ingest.ext" "$dest/ca.srl"
chmod 0600 "$dest/ca.key" "$dest/sor.key" "$dest/ingest.key"
chmod 0644 "$dest/ca.crt" "$dest/sor.crt" "$dest/ingest.crt"

printf '%s\n' 'PATHFINDER_C1_DEV_PKI_PASS validity_days=7 CA_key_must_remain_on_host=true dev_only=true jails_changed=0 SQL_writes=0'
printf '%s\n' 'REQUIRED: install only ingest.key + ingest.crt + ca.crt in ingest jail'
printf '%s\n' 'REQUIRED: install only sor.key + sor.crt + ca.crt in SoR jail'
printf '%s\n' 'NEVER install ca.key in either jail or commit it to Git.'
