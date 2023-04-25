# crust
`crust` helps you build and run all the applications needed for development and testing.

## Prerequisites
To use `crust` you need:
- `go 1.18` or newer
- `docker`

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
- 1cored - runs one cored validator (default one)
- 3cored - runs three cored validators (1cored and 3cored are mutually exclusive)
- faucet - runs faucet
- explorer - runs block explorer
- monitoring - runs the monitoring stack
- integration-tests - runs setup required by integration tests (3cored and faucet)

To start fully-featured set you may run:

```
$ crust znet start --profiles=3cored,faucet,explorer,monitoring
```

### --cored-version

The `--cored-version` allows to start the `znet` with any previously released version.

```
$ crust znet start --cored-version=v0.1.1 --profiles=3cored,faucet,explorer,monitoring
```

Also, it's possible to execute tests with any previously released version.

```
$ crust znet test --cored-version=v0.1.1 --test-groups=coreum-upgrade
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
- `ping-pong` - sends transactions to generate traffic on blockchain

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
(znet) [znet] $ cored-00 keys list
(znet) [znet] $ cored-00 query bank balances devcore1x645ym2yz4gckqjtpwr8yddqzkkzdpkt8nypky
(znet) [znet] $ cored-00 tx bank send bob devcore1x645ym2yz4gckqjtpwr8yddqzkkzdpkt8nypky 10core
(znet) [znet] $ cored-00 query bank balances devcore1x645ym2yz4gckqjtpwr8yddqzkkzdpkt8nypky
```

## Integration tests

Tests are defined in [crust/tests/index.go](crust/tests/index.go)

You may run tests directly:

```
$ crust znet test
```

Tests run on top of `--profiles=integration-tests`.

It's also possible to enter the environment first, and run tests from there:

```
$ crust znet --profiles=integration-tests
(znet) [znet] $ tests

# Remember to clean everything
(znet) [znet] $ remove
```

After tests complete environment is still running so if something went wrong you may inspect it manually.

## Ping-pong

There is `ping-pong` command available in `znet` sending transactions to generate some traffic on blockchain.
To start it run these commands:

```
$ crust znet
(znet) [znet] $ start
(znet) [znet] $ ping-pong
```

You will see logs reporting that tokens are constantly transferred.

## Hard reset

If you want to manually remove all the data created by `znet` do this:
- use `docker ps -a`, `docker stop <container-id>` and `docker rm <container-id>` to delete related running containers
- run `rm -rf ~/.cache/crust/znet` to remove all the files created by `znet`

## Monitoring

If you use the `monitoring` profile to start the `znet` you can open `http://localhost:3001` to access the Grafana UI (`admin`/`admin` credentials). 
Or use `http://localhost:9092` to access the prometheus UI.
