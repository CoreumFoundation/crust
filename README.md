# crust
`crust` helps you build and run all the applications needed for development and testing.

## Prerequisites
To use `crust` you need:
- `go 1.18` or newer
- `docker` (if you are installing from pacakge manager make sure that `docker-buildx` is installed.
    for docker version >23 it must be installed separately.
    Or you can alternatively follow the official installation guide to install docker-desktop
    , from this [link](https://docs.docker.com/engine/install/))

Install them manually before continuing.

## Building
1. Clone repo to the directory of your choice (let's call it `$COREUM_PATH`):
```
cd $COREUM_PATH
git clone https://github.com/CoreumFoundation/crust
```

2. Not required but recommended: Add `$COREUM_PATH/crust/bin` to your `PATH` environment variable:
```
export PATH="$COREUM_PATH/crust/bin:$PATH"
```

3. Compile all the required binaries and docker images:
```
$COREUM_PATH/crust/bin/crust build images
```

After the command completes you may find executable `$COREUM_PATH/crust/bin/cored`, being both blockchain node and client.


## Executing `znet`

`znet` is the tool used to spin up development environment running the same components which are used in production.

`znet` may be executed using two methods.
First is direct where you execute command directly:

```
$ crust znet <command> [flags]
```

Second one is by entering the znet-environment:

```
$ crust znet [flags]
(<environment-name>) [znet] $ <command> 
```

The second method saves some typing.

## Flags

All the flags are optional. Execute

```
$ crust znet <command> --help
```

to see what the default values are.

### --env

Defines name of the environment, it is visible in brackets on the left.
Each environment is independent, you may create many of them and work with them in parallel.

### --profiles

Defines the list of available application profiles to run. Available profiles:
- `1cored` - runs one cored validator (default one)
- `3cored` - runs three cored validators
- `5cored` - runs five cored validators mutually exclusive)
- `devnet` - runs environment similar to our devnet - 3 validators, 1 sentry node, 1 seed node, 2 full nodes
- `faucet` - runs faucet
- `explorer` - runs block explorer
- `monitoring` - runs the monitoring stack
- `integration-tests-ibc` - runs setup required by IBC integration tests
- `integration-tests-modules` - runs setup required by modules integration tests

NOTE: `1cored`, `3cored`, `5cored` and `devnet` are mutually exclusive.

To start fully-featured set you may run:

```
$ crust znet start --profiles=3cored,faucet,explorer,monitoring
```
**NOTE**: Notice from here on out, if you already have a znet env started with a set profiles,
and you want to start znet with a different set of profiles, you need to remove previous znet env
with `crust znet remove` and only then you can start the new env. 

### --cored-version

The `--cored-version` allows to start the `znet` with any previously released version.

```
$ crust znet start --cored-version=v1.0.0 --profiles=3cored,faucet,explorer,monitoring
```
**NOTE**: if you already have a znet env started with different profiles, you need to remove it 
with `crust znet remove` so you can start a new environment.

Also, it's possible to execute tests with any previously released version.

```
$ crust znet test --cored-version=v1.0.0 --test-groups=coreum-upgrade
```

## Commands

In the environment some wrapper scripts for `znet` are generated automatically to make your life easier.
Each such `<command>` calls `crust znet <command>`.

Available commands are:
- `start` - starts applications
- `stop` - stops applications
- `remove` - stops applications and removes all the resources used by the environment
- `spec` - prints specification of the environment
- `tests` - run integration tests
- `console` - starts `tmux` session containing logs of all the running applications

## Example

Basic workflow may look like this:

```
# Enter the environment:
$ crust znet
(znet) [znet] $

# Start applications
(znet) [znet] $ start

# Print spec
(znet) [znet] $ spec

# Stop applications
(znet) [znet] $ stop

# Start applications again
(znet) [znet] $ start

# Stop everything and clean resources
(znet) [znet] $ remove
$
```

## Logs

After entering and starting environment:

```
$ crust znet
(znet) [znet] $ start
```

it is possible to use `logs` wrapper to tail logs from an application:

```
(znet) [znet] $ logs cored-00
```

## Playing with the blockchain manually

For each `cored` instance started by `znet` wrapper script named after the name of the node is created, so you may call the client manually.
There are also three standard keys: `alice`, `bob` and `charlie` added to the keystore of each instance.

If you start `znet` using default `--profiles=1cored` there is one `cored` application called `cored-00`.
To use the client you may use `cored-00` wrapper:

```
(znet) [znet] $ start 
```

Generate a wallet to transfer funds to
```
(znet) [znet] $ cored-00 keys add {YOUR_WALLET_NAME}
```
Take the address the out put of the command above, you will use it in the next commands.

```
(znet) [znet] $ cored-00 query bank balances {YOUR_GENERATED_ADDRESS}
(znet) [znet] $ cored-00 tx bank send bob {YOUR_GENERATED_ADDRESS} 10udevcore
(znet) [znet] $ cored-00 query bank balances {YOUR_GENERATED_ADDRESS}
```

## Integration tests

You may run integration tests directly:

```
$ crust znet test
```

Tests run on top of `--profiles=integration-tests`.

It's also possible to enter the environment first, and run tests from there:

```
$ crust znet 
(znet) [znet] $ start --profiles=integration-tests
(znet) [znet] $ tests

# Remember to clean everything
(znet) [znet] $ remove
```

After tests complete environment is still running so if something went wrong you may inspect it manually.


## Hard reset

If you want to manually remove all the data created by `znet` do this:
- use `docker ps -a`, `docker stop <container-id>` and `docker rm <container-id>` to delete related running containers
- run `rm -rf ~/.cache/crust/znet` to remove all the files created by `znet`

## Monitoring

If you use the `monitoring` profile to start the `znet` you can open `http://localhost:3001` to access the Grafana UI (`admin`/`admin` credentials). 
Or use `http://localhost:9092` to access the prometheus UI.
