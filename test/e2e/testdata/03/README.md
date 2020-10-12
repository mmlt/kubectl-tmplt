This directory contains examples that uses Hashi Vault KV and PKI backends.

The `act` directory contains actions that read data from a central key vault and write it to Vault.
For testing we use values from `../keyvault` (normally similar values are fetched from secure place)

## PKI example
For the PKI example a `../filevault/xyz-cert` is created with

    openssl req -x509 -sha256 -nodes -days 365 -newkey rsa:2048 -keyout private.key -out cert.crt
    echo { \"cert\": \"$(sed 's/$/\\n/' cert.crt | tr -d '\n')\", \"key\": \"$(sed 's/$/\\n/' private.key | tr -d '\n')\" } > xyz-cert


Certs can be issued with: 
    vault write pkixyz/issue/foobarer common_name=my.foobar
and issued+decoded with: 
    vault write pkixyz/issue/default common_name=my.default -format=json | jq -r '.data.certificate' | openssl x509 -in - -text
