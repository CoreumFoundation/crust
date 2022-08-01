# WASM Contracts

Crust contains all-in-one set of commands to work with smart-contract development, targeting the Coreum chain, which is WASM enabled.

## Getting Started

* Make sure you have followed [Rust installation](https://www.rust-lang.org/tools/install) procedure and got a `stable` version
* Make sure you have `docker` (for optimized builds)
* Check also that you have a valid `CC`. Installing GCC or `build-essentials` into your system will provide it.

And most importantly, make sure you have [crust](/README.md) installed properly and ready to be used for smartcontract operations.

```bash
> crust contracts
Tools for the WASM smart-contracts development on Coreum

Usage:
   [flags]
   [command]

Available Commands:
  build       Builds the WASM contract located in the workspace dir. By default current dir is used.
  help        Help about any command
  init        Initializes a new WASM smart-contract project by cloning the remote template into target dir
  test        Runs unit-test suite in the workspace dir. By default current dir is used.

Flags:
  -h, --help                help for this command
      --log-format string   Format of log output: console | json (default "console")
  -v, --verbose             Turns on verbose logging

Use " [command] --help" for more information about a command.
```

## Creating a new smart-contract

`crust contracts init`

Creating a WASM-ready contract is just a bit more involved procedure, as compared to Solidity contracts, when you can have a single file and be happy. WASM contracts require to have Cargo configuration for dependency and target management, similar to NPM/YARN.

We are providing a template repo with bare minimum scaffolded code to help you be up and running with a new shiny contract. Crust provides an `contracts init` subcommand that clones the template into any target dir. Don't forget to specify a name!

```bash
> crust contracts init -h
Initializes a new WASM smart-contract project by cloning the remote template into target dir

Usage:
   init <target-dir> [flags]

Aliases:
  init, i, gen, generate

Flags:
  -h, --help                      help for init
      --project-name string       Specify smart-contract name for the scaffolded template
      --template-repo string      Public Git repo URL to clone smart-contract template from (default "https://github.com/CoreumFoundation/smartcontract-template.git")
      --template-subdir string    Specify a subfolder within the template repository to be used as the actual template
      --template-version string   Specify the version of the template, e.g. 1.0, 1.0-minimal, 0.16 (default "1.0")

```

### Example

```bash
> crust contracts init --project-name="test2" ./test2
🔧   Basedir: /Users/dev/coreum/contracts/test2 ...
🔧   Generating template ...
🔧   Moving generated files into: `/Users/dev/coreum/contracts/test2`...
✨   Done! New project created /Users/dev/coreum/contracts/test2

> ls /Users/dev/coreum/contracts/test2
Cargo.lock     Developing.md  LICENSE        Publishing.md  examples/      schema/
Cargo.toml     Importing.md   NOTICE         README.md      rustfmt.toml   src/
```

## Testing a smart contract

`crust contracts test`

In CosmWasm there are three approaches to testing:
1) Unit tests
2) Integration test
3) Integration tests that are loading .wasm artefact as a black box

We decided to follow the same route and developed a crust subcommand that helps you to run all unit and integration tests in the project workspace:

```bash
> crust contracts test -h
Runs unit-test suite in the workspace dir. By default current dir is used.

Usage:
   test [workspace-dir] [flags]

Aliases:
  test, t, unit-test

Flags:
      --coverage      Enable code coverage report using tarpaulin (Linux x64 / MacOS x64 / M1).
  -h, --help          help for test
      --integration   Enables the integration tests stage.

```

### Example

A simple unit test running looks like this:

```bash
> crust contracts test ./test2

   Compiling test2 v0.1.0 (/Users/dev/coreum/contracts/test2)
    Finished test [unoptimized + debuginfo] target(s) in 5.75s
     Running unittests src/lib.rs (target/debug/deps/test2-17ec6b87946f85ec)

running 4 tests
test contract::tests::proper_initialization ... ok
test contract::tests::reset ... ok
test contract::tests::increment ... ok
test integration_tests::tests::count::count ... ok

test result: ok. 4 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.00s
```

If you are lucky to have **Linux x86_64** or **MacOS Intel x64 / M1** platforms at hand, you can run the unit testing with reported coverage:

```bash
> crust contracts test --coverage ./test2
...
test result: ok. 4 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.01s

Jul 19 17:57:00.735  INFO cargo_tarpaulin::report: Coverage Results:
|| Tested/Total Lines:
|| src/contract.rs: 64/65
|| src/helpers.rs: 8/14
|| src/integration_tests.rs: 24/24
||
93.20% coverage, 96/103 lines covered

[INFO]	contracts	contracts/test.go:171	Code coverage report written	{"path": "./test2/coverage/tarpaulin-report.html"}
```

## Building a smart contract

`crust contracts building`

Now, when it comes to building a smart-contract, there are two options and two workflows that are implemented in crust. First option will use your Rust environment to build a Release version of the smart-contract targeting WASM32. This mode doesn't need Docker, but the resulting artefact will be large and the build is not deterministic.

Second option is to have a deterministic and optimized build through a special Docker image. `rust-optimizer` produces reproducible builds of WASM smart contracts. This means third parties can verify that the contract is actually the claimed code.

```bash
> crust contracts build -h
Builds the WASM contract located in the workspace dir. By default current dir is used.

Usage:
   build [workspace-dir] [flags]

Aliases:
  build, b

Flags:
  -h, --help        help for build
      --optimized   Enables WASM optimized build using a special Docker image, ensuring minimum deployment size and predictable execution.
```

### Both examples

```bash
> crust contracts build ./test2
...

   Compiling test2 v0.1.0 (/home/xlab/docker/cw-crust/test2)

    Finished release [optimized] target(s) in 18.86s
[INFO]	contracts	contracts/build.go:73	Build artefact created	{"name": "test2", "dir": "./test2", "path": "./test2/target/wasm32-unknown-unknown/release/test2.wasm"}
```

Output binary `wasm32-unknown-unknown/release/test2.wasm` is **1.8M** in size.

```bash
$ crust contracts build --optimized ./test2

Info: RUSTC_WRAPPER=sccache
Info: sccache stats before build
...
Cache location                  Local disk: "/root/.cache/sccache"
Cache size                            0 bytes
Max cache size                       10 GiB
Building contract in /code ...
...
   Compiling test2 v0.1.0 (/code)
    Finished release [optimized] target(s) in 43.22s
Creating intermediate hash for test2.wasm ...
0f4c02b49e03c1cc5ba0fed56774dca1dd89fd41010037f6f2baf57dc203261a  ./target/wasm32-unknown-unknown/release/test2.wasm
Optimizing test2.wasm ...
Creating hashes ...
6e6e09439b23b06d03b42b45f270aaa7eb7236f553d88daa018ad60c9254b592  test2.wasm
Info: sccache stats after build
...
Cache location                  Local disk: "/root/.cache/sccache"
Cache size                           17 MiB
Max cache size                       10 GiB
done

[INFO]  contracts   contracts/build.go:73   Build artefact created  {"name": "test2", "dir": "./test2", "path": "./test2/artifacts/test2.wasm"}
```

Output binary `./test2/artifacts/test2.wasm` is **132K** in size ⚠️

## Deploying a smart contract

WIP
