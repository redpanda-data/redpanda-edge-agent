#!/usr/bin/env bash

CERTS_DIR=certs

if [ -d $CERTS_DIR ]
then
    echo "Delete existing $CERTS_DIR directory before running this script"
    exit 1
fi

mkdir -p $CERTS_DIR

# Create OpenSSL CA configuration file
cat > ca.cnf <<EOF
# OpenSSL CA configuration file
[ ca ]
default_ca         = CA_default
[ CA_default ]
default_days       = 365
database           = index.txt
serial             = serial.txt
default_md         = sha256
copy_extensions    = copy 

# Used to create the CA certificate
[ req ]
prompt             =no
distinguished_name = distinguished_name
x509_extensions    = extensions
[ distinguished_name ]
organizationName   = Redpanda
commonName         = Redpanda CA
[ extensions ]
keyUsage           = critical,digitalSignature,nonRepudiation,keyEncipherment,keyCertSign
basicConstraints   = critical,CA:true,pathlen:1

# Common policy for nodes and users.
[ signing_policy ]
organizationName   = supplied
commonName         = optional

# Used to sign node certificates.
[ signing_node_req ]
keyUsage           = critical,digitalSignature,keyEncipherment
extendedKeyUsage   = serverAuth,clientAuth

# Used to sign client certificates.
[ signing_client_req ]
keyUsage           = critical,digitalSignature,keyEncipherment
extendedKeyUsage   = clientAuth
EOF

# Generate RSA private key for the CA
openssl genrsa -out $CERTS_DIR/ca.key 2048
chmod 400 $CERTS_DIR/ca.key

# Generate self-signed CA certificate
openssl req -new -x509 -config ca.cnf -key $CERTS_DIR/ca.key -out $CERTS_DIR/ca.crt -days 365 -batch

rm -f index.txt serial.txt
touch index.txt
echo '01' > serial.txt

# Create OpenSSL node and client configuration file
cat > node.cnf <<EOF
# OpenSSL node configuration file
[ req ]
prompt             = no
distinguished_name = distinguished_name
req_extensions     = extensions
[ distinguished_name ]
organizationName   = Redpanda
[ extensions ]
subjectAltName     = critical,@alt_names
[ alt_names ]
DNS  = 127.0.0.1
IP.1 = 127.0.0.1
IP.2 = 172.24.1.10
IP.3 = 172.24.1.20
EOF

# Generate RSA private key for the nodes
openssl genrsa -out $CERTS_DIR/src.key 2048
chmod 400 $CERTS_DIR/src.key

openssl genrsa -out $CERTS_DIR/dst.key 2048
chmod 400 $CERTS_DIR/dst.key

# Generate Certificate Signing Request (CSR) for the nodes
openssl req -new -config node.cnf -key $CERTS_DIR/src.key -out $CERTS_DIR/src.csr -batch
openssl req -new -config node.cnf -key $CERTS_DIR/dst.key -out $CERTS_DIR/dst.csr -batch

# Use the CA to sign the CSR for the nodes
openssl ca \
-config ca.cnf \
-keyfile $CERTS_DIR/ca.key \
-cert $CERTS_DIR/ca.crt \
-policy signing_policy \
-extensions signing_node_req \
-in $CERTS_DIR/src.csr \
-out $CERTS_DIR/src.crt \
-outdir $CERTS_DIR/ \
-subj "/O=redpanda/CN=redpanda_source" \
-batch

openssl ca \
-config ca.cnf \
-keyfile $CERTS_DIR/ca.key \
-cert $CERTS_DIR/ca.crt \
-policy signing_policy \
-extensions signing_node_req \
-in $CERTS_DIR/dst.csr \
-out $CERTS_DIR/dst.crt \
-outdir $CERTS_DIR/ \
-subj "/O=redpanda/CN=redpanda_destination" \
-batch

# Generate RSA private key for the agent
openssl genrsa -out $CERTS_DIR/agent.key 2048
chmod 400 $CERTS_DIR/agent.key

# Generate Certificate Signing Request (CSR) for the agent
openssl req -new -config node.cnf -key $CERTS_DIR/agent.key -out $CERTS_DIR/agent.csr -batch

# Use the CA to sign the CSR for the agent
openssl ca \
-config ca.cnf \
-keyfile $CERTS_DIR/ca.key \
-cert $CERTS_DIR/ca.crt \
-policy signing_policy \
-extensions signing_node_req \
-in $CERTS_DIR/agent.csr \
-out $CERTS_DIR/agent.crt \
-outdir $CERTS_DIR/ \
-subj "/O=redpanda/CN=redpanda_agent" \
-batch

# Set permissions required for Docker on Linux
chmod -R 755 certs

# Cleanup
rm -f *.cnf index.txt* serial.txt* $CERTS_DIR/*.pem $CERTS_DIR/*.csr