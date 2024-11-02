# Packet Shell

The packet shell is a command-line-based utility for creating, sending,
receiving, viewing, and managing Santa Clara County ARES®/RACES packet messages.
It is completely interoperable with the standard SCCo packet software ([Outpost]
and [PackItForms]), but it is not derived from them or dependent on them.

[Outpost]: http://www.outpostpm.org/
[PackItForms]: https://www.scc-ares-races.org/data/packet/about-packitforms.html

The packet shell has been tested on Windows, Mac OS, and Linux. It is suitable
for both human and scripted use. It has no external dependencies, and does not
require an installer. It can be run from removable media.

The packet shell is highly opinionated software. It enforces and streamlines
the SCCo ARES®/RACES packet procedures. The ability to use this software in any
context other than SCCo ARES®/RACES is coincidental and not guaranteed.

The packet shell is based around “incidents,” which are groups of messages
tracked in the same ICS-309 communications log. Each incident has its own
directory where the messages, logs, and configuration for that incident are
stored. Each time you want to start a new ICS-309, you create a new incident
directory. Except for PDFs, all of the files in the incident directory are
plain text files, easily manipulated by other tools. For details of the
directory layout, run `packet help files`.

## Usage

The packet shell has a number of subcommands, such as `new`, `edit`, `connect`,
etc. These can be run one at a time as arguments to the packet shell:

    $ packet new ICS213
    $ packet edit XND-123P
    $ packet connect

Or you can run the `packet` command without arguments and then enter the
commands in the packet shell:

    $ packet
    packet> new ICS213
    packet> edit XND-123P
    packet> connect

Usually scripts will take the first approach and humans will take the second.
The result is the same either way. Note that commands and arguments can often
be abbreviated. This is equivalent:

    $ packet
    packet> n i
    packet> e 123
    packet> c

For a list of all commands, run `packet help`. For the details of a specific
command, run `packet help «command»`.

The input and output of the packet commands varies depending on whether standard
input and output are a terminal. When they are — i.e., when a human is running
the packet shell — the results are colorized, formatted for human eyes, and
interactive. When they are not — i.e., when a script is calling the packet
shell, or I/O redirection is in use — the results are script-friendly, terse,
and noninteractive. Run `packet help script` for more details.

A typical pattern for a new incident would start like this:

    (1)  $ mkdir 2023-07-MPMP
    (2)  $ cd 2023-07-MPMP
    (3)  $ packet
    (4)  packet> bulletins xscevent xnd@xsc     # or "b"
    (5)  packet> new check-in                   # or "n ci"
    (6)  packet> connect                        # or "c"

Here's an explanation, line by line:

1.  Create a new directory to hold the files for the new incident.
2.  Move into that directory so that it is the current working directory when
    running the `packet` command.
3.  Run the `packet` command without arguments to start the shell.
4.  Tell the `packet` command which bulletin areas to check. Since we didn't
    specify otherwise, it will check them once an hour.
5.  Create a new check-in message. The command will prompt the user for each of
    the fields of the check-in message, with full editing capability.
6.  Connect to the BBS. This will send the check-in message, retrieve any
    private messages, and retrieve bulletins from the two areas listed.

From that point, the user can do many different things. They could, for
example, read any messages or bulletins received, either in tabular form (the
`show` command) or in PDF form (the `pdf` command). Or they could create
additional outgoing messages (the `new` command). Or they could generate an
ICS-309 log for the incident (the `ics309` command). For a list of the
possible commands, run `packet help` (or just `help` if already in the packet
shell).

For a complete example of a monthly packet practice session using the packet
shell, see <a href="https://rothskeller.net/2023-12-MPMP.pdf">this file</a>.

## Installation

**For Linux / amd64:**

1. Download the pre-built `linux-amd64` binary from the [releases page](../../releases).
2. Rename it to `packet` and change its mode to `0755`.
3. Optional: move it to a directory that is on your `$PATH`.

**For Mac OS / amd64:**

1. Download the pre-built `macos-amd64` binary from the [releases page](../../releases).
2. Rename it to `packet` and change its mode to `0755`.
3. Optional: move it to a directory that is on your `$PATH`.

**For Windows / amd64:**

NOTE: The packet shell is compatible with Windows 10 (version 1511) or later.
It will not work on any earlier version of Windows.

1. Download the pre-built `windows-amd64` binary from the [releases page](../../releases).
2. Rename it to `packet.exe`.
3. Optional: move it to a directory that is on your `$PATH`.

**For other OSs or architectures:**

1. Install the [Go programming language](https://go.dev/), version 1.21.0 or
   later.
2. Run the command
   `$ go install -tags packetpdf github.com/rothskeller/packet-shell/packet`

This will download the code, build it for your system, and install it at
`$HOME/go/bin/packet`.

### Omitting PDF Generation Support

The packet shell includes support for generating PDF versions of messages and
ICS-309 logs. If you don’t need that — for example, if you are using the packet
shell only in scripts — you can use a version without PDF generation support.
It is substantially smaller than the full version and may be a bit faster. If
you want that, follow the “For other OSs or architectures” instructions above,
but omit the `-tags packetpdf` from the build command.

### Private Forms

The packet shell available through this site understands only the public SCCo
ARES®/RACES forms. If you need a version that understands the private forms as
well, it is available to authorized people. Contact the author for access.

## Limitations

Currently, the packet shell only supports two BBS connection methods: RF via
Kantronics KPC-3 Plus TNC and radio, or Internet via telnet. Other methods —
in particular, other TNC models — could be added if someone wants to loan the
author the hardware needed to test them.

## Legal Text

This software was written by Steve Roth, KC6RSC.

Copyright © 2023–2024 by Steven Roth <steve@rothskeller.net>

See LICENSE.txt for license details.
