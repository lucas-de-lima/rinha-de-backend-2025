# Gera CA, certificados de servidor e cliente para mTLS local (PowerShell)
$ErrorActionPreference = 'Stop'
$certsPath = $PSScriptRoot
$opensslCnf = Join-Path $certsPath 'openssl.cnf'

# CA
openssl genrsa -out (Join-Path $certsPath 'ca.key') 4096
openssl req -x509 -new -nodes -key (Join-Path $certsPath 'ca.key') -sha256 -days 3650 -out (Join-Path $certsPath 'ca.crt') -subj "/CN=RinhaCA" -config $opensslCnf

# Servidor
openssl genrsa -out (Join-Path $certsPath 'server.key') 4096
openssl req -new -key (Join-Path $certsPath 'server.key') -out (Join-Path $certsPath 'server.csr') -subj "/CN=localhost" -config $opensslCnf -extensions v3_req
openssl x509 -req -in (Join-Path $certsPath 'server.csr') -CA (Join-Path $certsPath 'ca.crt') -CAkey (Join-Path $certsPath 'ca.key') -CAcreateserial -out (Join-Path $certsPath 'server.crt') -days 3650 -sha256 -extensions v3_req -extfile $opensslCnf

# Cliente
openssl genrsa -out (Join-Path $certsPath 'client.key') 4096
openssl req -new -key (Join-Path $certsPath 'client.key') -out (Join-Path $certsPath 'client.csr') -subj "/CN=RinhaClient" -config $opensslCnf
openssl x509 -req -in (Join-Path $certsPath 'client.csr') -CA (Join-Path $certsPath 'ca.crt') -CAkey (Join-Path $certsPath 'ca.key') -CAcreateserial -out (Join-Path $certsPath 'client.crt') -days 3650 -sha256 -extfile $opensslCnf -extensions v3_req

Remove-Item (Join-Path $certsPath '*.csr'), (Join-Path $certsPath '*.srl') -ErrorAction SilentlyContinue
