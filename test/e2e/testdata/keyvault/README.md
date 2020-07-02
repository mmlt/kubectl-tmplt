This directory contains the config files to access Azure KeyVault.

Each file contains a single configuration parameter.

Files:
- `type` is the type of vault `azure-key-vault` or `file`
- `cli` (optional, development only) contains 'true' to use your `az logn` credentials to access KeyVault instead of `AZURE_*` values.

Other files correspond to the environment variables documented in
[azure-sdk-go-authorization](https://docs.microsoft.com/en-us/azure/go/azure-sdk-go-authorization#use-environment-based-authentication)

