pkgupd
======

About
-----

**Pkugpd** is a daemon that monitors archlinux repositories and AUR for
updates.  The server understands simple JSON messages and clients can be
written in any language.  A simple CLI client written in python is included,
which also serves as the reference client implementation.

This project is a rewrite of [pkgupdate](https://github.com/foucault/pkgupdate).
Hopefully, it performs better and is easier to maintain.

HowTo
-----

### Compile and install

You will need a Go toolchain to build. In your `$GOPATH/src` clone the
repository (it's not go-gettable yet, sorry) and also `go get` the dependencies

    go get github.com/jessevdk/go-flags
    go get github.com/go-fsnotify/fsnotify

Then you can `go install pkugpd/pkgupd`. It is recommended that you use the
[PKGBUILD](https://aur.archlinux.org/packages/pkgupd-git) for the installation
though, as it will build a ready-to-use archlinux package.

### Use and configure

A systemd service is bundled with the package with reasonable defaults. You can
tweak them by editing `/etc/conf.d/pkgupd` and adding your options in
`$PKGUPD_ARGS`. Check the manpage for all available options.

Communicating with the server
-----------------------------

Clients can poll the server for updates either through a TCP socket or through
a UNIX socket, depending on how the program has been invoked. The protocol is
very simple, but it may change in the future.  To ask for an update the client
sends a JSON string as such. JSON strings **must** be followed by a new line
character `\n`. Everything after the new line character is discarded.

    { "RequestType": "[ServiceType]" }\n

`[ServiceType]` can be currently either `repo` or `aur` for, although more can
be added in the future. `repo` prints all updatable packages that are backed by
a repository and `aur` all updatable packages that are found in AUR.  Packages
missing from either services are never reported.

The format of the server's response is

    { "ResponseType": "[Type]", "Data": "..." }\n

`[Type]` can either be `ok` or `error`. In case of an error `Data` contains the
error message. If no error occurred `Data` will be a **list** of packages. The
format of a package is:

    {
      "Name" : "...",
      "LocalVersion" : "...",
      "RemoteVersion" : "...",
      "Foreign" : "[true|false]"
    }

New lines are added for clarity. There are no new lines in the response except
for the final one, so you can safely delimit the responses at new lines.
`Name` is the name of the package, `LocalVersion` is the currently installed
version of the package, `RemoteVersion` is the updatable version either on the
repository or in AUR and `Foreign` indicates whether the package is backed by a
repository (`false`) or not (`true`).

Bugs
----
If you find a bug, open an issue, or better yet send in a pull request.

License
-------
This project is licensed under the
[GPLv3](https://www.gnu.org/licenses/gpl-3.0.html) or any newer.

