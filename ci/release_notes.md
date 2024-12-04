### Improvements

- Document why Alpine image can't be used for building to Go code
- Deprecate the specific `skip_download` resource property in favor of the standard `no_get` [Concourse feature][no_get] (submitted in #10, thanks @brakel!)
- Built with latest go 1.21 patch version (i.e. v1.21.13)

[no_get]: https://concourse-ci.org/put-step.html#schema.put.no_get
