# Build system

## Motivation

We need a library to automate common tasks:
- building binaries
- building docker images
- linting
- testing
- releasing
- installing development tools

Most common approach is to use `Makefile`. This approach would introduce a lot of bash code to our project.
The other option is to write everything in go to keep our code as consistent as possible.
Moreover, we are much better go developers than bash ones.

So here is the simple tool written in go which helps us in our daily work.

## Shell configuration
It is assumed here that cloned `crust` repository exists in `$HOME/crust`.
If you store it elsewhere, adjust paths accordingly.

Configuration for two shells is provided: `bash` and `zsh`.

Once configuration is done log out from your session and log in again to apply changes.

### Bash
Add this line at the end of `~/.bash_profile` file:

```
PATH="$HOME/crust/bin:$PATH"
```

If you want to use autocompletion feature of `crust` add this line at the end of `~/.bashrc` file:

```
complete -o nospace -C crust crust 
```

### ZSH

Add this line at the end of `~/.zprofile` file:

```
PATH="$HOME/crust/bin:$PATH"
```


## Installing tools

Run

```
$ crust setup
```

to install all the essential tools we use.

Whenever tool downloads or builds binaries it puts them inside [bin](../bin) directory so they are
easily callable from console.

## `crust` command

`crust` command is used to execute operations. you may pass one or more operations to it:

`crust <op-1> <op-2> ... <op-n>`

Here is the list of operations supported at the moment:

- `setup` - install all the essential tools required to develop our software
- `lint` - runs code linter
- `tidy` - executes `go mod tidy`
- `test` - runs unit tests

If you want to inspect source code of operations, go to [build/index.go](index.go). 

You may run operations one by one:

```
$ crust lint
$ crust test
```

or together:

```
$ crust lint test
```

Running operations together is better because if they internally have common dependencies, each of them will
be executed once. Moreover, each execution of `crust` may compile code. By running more operations at once
you just save your time. In all the cases operations are executed sequentially.

To remove all the caches and docker containers and images used by crust run:

```
crust remove
```

## Common environment

The build tool is also responsible for installing external binaries required by our environment.
The goal is to keep our environment consistent across all the computers used by our team members.

So whenever `go` binary or anything else is required to complete the operation, the build tool ensures
that correct version is used. If the version hasn't been installed yet, it is downloaded automatically for you.
