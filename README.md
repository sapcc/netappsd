

## convert gopkg to go module
1. make sure gopkg.lock file is present
2. go mod init [module path]: This will import dependencies from Gopkg.lock.
3. go mod tidy: This will remove unnecessary imports, and add indirect ones.
4. rm -f Gopkg.lock Gopkg.toml: Delete the obsolete files used for Dep.

ref: https://stackoverflow.com/a/55664631
