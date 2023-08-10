# “Packet” Shell

The packet shell combines a number of packet related commands into a single
executable.  Available commands include:

    packet bulletins  schedule and perform bulletin checks
    packet configure  set incident/activation and connection parameters
    packet connect    connect, send queued messages, and receive messages
    packet delete     delete an unsent message completely
    packet draft      remove an unsent message from the send queue
    packet dump       show a message in encoded form
    packet edit       edit an unsent outgoing message
    packet help       provide help on the packet shell or its commands
    packet ics309     generate an ICS-309 form with all messages
    packet list       list messages
    packet new        create a new outgoing message
    packet queue      queue an unsent message to be sent
    packet quit       quit the packet shell
    packet set        change a field value in an unsent, outgoing message
    packet show       show a message

Running the `packet` command with no arguments starts a shell, in which multiple
packet commands can be run without the `packet` prefix word.

Most of the commands take a «message-id» as an argument, identifying the message
to act on.  Messages can be identified by either their local message ID or, if
known, their remote message ID.  The prefix and/or suffix can be left off of the
message ID if not needed for uniqueness, so usually messages are identified by
just the sequence number part of the message ID.  Leading zeros are not required
or significant.

Some of the commands take a «field-name» as an argument, identifying a specific
field of the message.  The «field-name» can be either the PackItForms tag for
the field (including any trailing period), or a shortened version of the field
name as shown in the `show` and `edit` commands.  The `packet` command uses
heuristics to select the best matching field for whatever name is given.  (Hint:
string together all of the capital letters of the field name, like `tl` for “To
Location”.)

## Configure Command

The `configure` (or `conf` or `config`) command displays or sets the value of
settings for the incident or activation (i.e., all invocations of `packet` in
the same working directory).

    usage: packet configure [«name» [=] [«value»]]

With no arguments, it displays the values of all settings.  With only a setting
name, it displays the value of that setting.  With a name and value, it sets
that setting to that value.

When a packet command needs a value for a setting that has not yet been set, it
will prompt the user for it.

Known settings are:

    incident    incident name
    activation  activation number
    period      operational period
    operator    operator call sign and name
    tactical    tactical station call sign and name
    bbs         BBS to connect to
    tnc         serial port of TNC
    password    password for logging into BBS
    msgid       starting local message ID
    defbody     default body string for new messages

### `incident`, `activation`, and `period` Settings

The `incident`, `activation`, and `period` settings provide the incident name,
activation number, and operational period for generated ICS-309 forms.  The
incident name and activation number are free-form text.  The operational period
has the form

    MM/DD/YYYY HH:MM MM/DD/YYYY HH:MM

giving the start and end dates and times.  The ending date can be omitted if it
is the same as the starting date.

### `operator` and `tactical` Settings

The `operator` and `tactical` settings provide the identification of the radio
operator and the tactical station, respectively.  An `operator` setting is
required, but a `tactical` setting is optional.  Both of them have the form

    callsign name

When packet commands connect to a BBS, they use the tactical call sign if
set, otherwise the operator call sign.  (If a tactical call sign is used, proper
FCC identification with the operator call sign does occur.)

The operator and tactical station identification are also used to populate
fields of newly created, sent, or received messages.

### `bbs`, `tnc`, and `password` Settings

The `bbs` setting gives the address of the BBS to which to connect.  For TNC/RF
connections, it is an AX.25 address (e.g., `W6XSC-1`).  For telnet connections,
it is a hostname:port string (e.g., `w6xsc.ampr.net:8080`).  Note that the
hostname must start with the BBS call sign.

For TNC/RF connections, the `tnc` setting must be set to the serial port through
which to communicate with the TNC.  On Windows systems, this will be `COM#`,
where `#` is some number.  The `packet` command will guess `COM3` until told
otherwise.  On other systems, this will be the path to a character device file.
The `packet` command will guess the highest-numbered `/dev/tty*USB*` file.

For telnet connections, the `password` setting must be set to the password
needed to log into the BBS.  Note that this password is stored in clear text in
`packet.conf` in the current working directory; take care to ensure it is
properly protected.

### `msgid` Setting

The `msgid` setting gives the local message ID pattern and the initial message
ID.  Newly received incoming messages will be given a message number following
this model.  Unless overridden while editing, newly created will also.

### `defbody` Setting

If the `defbody` setting has a value, its value is placed into the primary body
text field of any newly created outgoing message.  The primary use for this is
adding `**** This is drill traffic ****` or similar strings to all messages.

## New Command

The `new` (or `n`) command creates a new outgoing message, and opens that
message in an editor window (see “Edit Command” below).  The new message is
given default values for all fields.

    packet new [--reply «message-id»] [--copy «message-id»] [«message-type»]

The message type must be the tag of a known message type, such as `plain` or
`ICS213`.  The tags are not case-sensitive, and can be abbreviated as long as
they are unique.  (`ci` and `co` are accepted as aliases for `Check-In` and
`Check-Out`, for convenience.)

## Reply Command

The `reply` command creates a new outgoing message as a reply to an existing
received message.  It opens the new outgoing message in an editor window (see
“Edit” below).

    packet reply «received-message-ID» [«message-type»]

If a message type is specified, it must be the tag of a known message type, such
as `plain` or `ICS213`.  The tags are not case-sensitive, and can be abbreviated
as long as they are unique.  If no message type is specified, the created
message will be of the same type as the received message.

The new message will have the same handling order, subject, and (when
applicable) body as the received message.  The “To” address of the new message
will be the “From” address of the received message.  If the new message type has
a “Reference” field, it is set to the origin message ID of the received message.

## Edit Command

The `edit` (or `e`) command edits the contents of the message.  The keyword
`edit` can be omitted, since edit is the default action for unsent messages.

    packet edit «message-id» [«field-name»|errors]

The message editor prompts for each field of the message in order, allowing them
to be edited.  Use Tab and Shift-Tab to move backward and forward in the list of
fields.  (Enter usually moves forward also, except in multi-line text fields,
where it introduces a newline.)  Editing ends when you finish the last field or
press the ESC key.  Press F1 for help on the current field.

If a «field-name» is specified, editing begins with that field rather than the
start of the form.  (See the discussion of «field-name»s at the top of this page
for details.  Editing starts with the first field if the specified field name
doesn't match anything.

If the keyword `errors` is provided, editing includes only those fields that
have validation errors.

If the result of the edit has no validation errors, and the message is  not
already in the send queue, the message editor asks whether to queue it.
Messages with validation errors are removed from the send queue.

## Set Command

The `set` command changes the value of a field of an unsent message.

    packet set «message-id» «field-name» [«value»|<«filename»]

«message-id» identifies the message to be edited, and «field-name» identifies
the field of the message to be changed.  See the top of this page for how to
format these arguments.  If a «value» is specified, the field is set to that
value.  If a less-than sign is specified, followed by a «filename», the field
will be set to the contents of that file.  If neither is specified, the field
value is cleared.

## Queue, Draft, and Delete Commands

The `queue` command queues an unsent message to be sent.  The `draft` command
removes an unsent message from the send queue, leaving it in draft state.  The
`delete` command deletes an unsent message completely.  None of these commands
can be abbreviated.

    packet queue «message-id»
    packet draft «message-id»
    packet delete «message-id»

Note that unlike all other commands, the `delete` command does not allow the
«message-id» to be just a sequence number.  It must be fully written out.

## Connect, Send, and Receive Commands

The connection commands connect to a BBS to send and/or receive messages.

    packet connect|send|receive [immediate]

The `send` command sends queued messages, the `receive` command receives
incoming messages and performs scheduled bulletin checks, and the `connect`
command does both.  These can be abbreviated `s`, `r`, and `c`, respectively.
`sr` and `rs` are also accepted for performing both operations.

When the `immediate` (or `i`) keyword is present, only messages with immediate
handling are sent and/or received, and no bulletin checks are performed.  In
abbreviated form, the `i` keyword can be combined with the command word, so
`si`, `ri`, `ci`, `sri`, and `rsi` are all valid commands.

When receiving messages, the `connect` command automatically assigns the local
message ID based on the `msgid` setting (see “Configure Command” above), and it
fills in operator name and call sign fields of forms messages with the
`operator` setting.

## Bulletins Command

The `bulletins` (or `b`) command schedules checks for bulletins.  It updates the
schedules for bulletin checks, and then connects to the BBS and checks for new
bulletins.

    packet bulletins
    packet bulletins [«frequency»] «area»...

When `bulletins` is run without arguments, the connection will check for new
bulletins in all areas that have a schedule, even if their next check isn't due
yet.

When there are «area»s listed on the `bulletins` command, the schedules for
bulletin checks in those «area»s are changed to have the specified «frequency»
(which defaults to `1h`).  Then the connection will check for new bulletins in
only those areas that are due for a check according to the new schedule.

Each «area» must be a bulletin area name (e.g., `XSCEVENT`), optionally preceded
by a recipient name and an at-sign (e.g., `XND@ALLXSC`).

The «frequency» specifies how frequently the listed bulletin areas should be
checked for new bulletins, formatted like `30m` or `2h15m`.  Setting the
frequency to `0`` removes the scheduled checks for the listed areas.

## List Command

The `list` (or `l`) command gives a list of messages stored in the current
directory.  When running the `packet` command in shell mode, this command is run
when an empty command line is entered.

    packet list

The list will include received messages, sent messages, and unsent outgoing
messages.  They are listed in chronological order.  The list does not include
receipt messages.

## Show Command

The `show` (or `s`) command displays a message or message field.  If the message
has been sent or received, the `show` keyword can be omitted, since show is the
default action for all messages except unsent messages.

    packet show «message-ID» [«field-name»] [>«filename»]
    packet pdf «message-ID» 
    packet raw «message-ID»

«message-id» is the local or remote message ID of the message to show.  It can
be just the numeric part if that is unambiguous.  If a «field-name» is
specified, only the value of that field is printed; otherwise, all fields are
printed in a flat text table layout.  If a «filename» is provided, following a
">" character, the output will be written to the named file.

The `pdf` command renders the message in a PDF format (if it is of a type that
supports PDF rendering), and opens it in the system PDF viewer.

The `raw` command displays the message in the PackItForms and RFC-5322 encoding
as it would be sent over the air.

## ICS-309 Command

The `ics309` command generates and shows an ICS-309 communications log with
information from all of the sent and received messages stored in the current
directory, including receipts.  (Unsent messages are not included.)  The command
can be abbreviated `309`, but only if no message exists with that sequence
number in its message ID.  The updated form is opened in the system PDF viewer.

Note that the generated log is automatically removed whenever anything changes
that invalidates its contents (e.g., sending or receiving a message).  Run the
command again to regenerate it.

## Directory Contents

The `packet` commands work with messages in the current working directory where
it is invoked.  Ideally a separate directory is used for each incident or
activation.  In other words, start a new directory any time you would start a
new ICS-309 form.

Within this directory, each sent, unsent, or received message (other than
receipts) is stored in a file named with its local message ID and a `.txt`
extension.  If a remote message ID is known for that message, a symbolic link
with the remote message ID and a `.txt` extension is created, pointing to the
message file.  Within those files, messages are stored in encoded RFC-5322
format.  Received messages have a `Received:` header, sent messages have a
`Date:` header, and unsent messages have neither.

Forms messages that can be rendered in PDF format are automatically rendered
whenever they are changed using a `packet` command.  The PDF versions are named
the same way, with `.pdf` extensions.

Delivery and read receipts are stored in files named with the local message ID
and `.DR.txt` or `.RR.txt` extensions, respectively.  No symbolic links with
remote message IDs are created for them.

Settings for the incident or activation (see “Configure Command” above) are
stored in the file `packet.conf`, in JSON format.  Some settings that are likely
to be the same for all incidents are also stored in `$HOME/.packet` and used to
seed the settings for new incidents.

If an ICS-309 form has been generated, it is stored in CSV format in
`ics309.csv`, and in PDF format in `ics309.pdf`.  No ICS-309 forms are generated
until the `ics309` command is run.  Once generated, however, the ICS-309 forms
are kept up to date whenever any message is changed.

All communications with the BBS are logged in `packet.log`.
