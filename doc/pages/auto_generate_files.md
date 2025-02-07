### Auto generate files for Dependency Injection

In sqly, Dependency Injection is performed using `google/wire`. Initialization functions for each struct are aggregated in the `wire.go` file, and initialization functions for all packages are aggregated in the `di` package. To automatically generate the initialization code, please run the following command:

```shell
go generate ./...
```

or

```shell
make generate
```

### Auto generate files for Mock

In sqly, Mock is performed using `mockgen`. Mock files are generated in the `mock` package. To automatically generate the mock code, please run the following command:

```shell
go generate ./...
```

or

```shell
make generate
```
