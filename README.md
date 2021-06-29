# File Exporter

Prometheus Exporter for Files.

Inspired by [filestat_exporter](https://github.com/michael-doubez/filestat_exporter).

This project is a little more simplistic and is configurable entirely from the command line, it also monitors for filesystem events and updates the metrics using a library.

It exposes file modified time and the CRC32 hash of the file because it can be represented in float64 which then can be used as a value to a gauge metric in prometheus directly.

## Usage

```bash
file_exporter --path path/to/a/file/or/directory
```

## Help

If you do not specify a command, the default is `server`, so `file_exporter --path /tmp` and `file_exporter server --path /tmp` are equivalent.

```man
NAME:
   files_exporter - file_exporter

USAGE:
   files_exporter [global options] command [command options] [arguments...]

VERSION:
   1.0.0

AUTHOR:
   Erik Kristensen <ekristensen@sans.org>

COMMANDS:
   server   server
   version  print version
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --telemetry.addr value              Host and port to listen on (default: "0.0.0.0:9183") [$TELEMTRY_ADDR]
   --telemetry.path value              Path to listen for telemetry on (default: "/metrics") [$TELEMETRY_PATH]
   --path value, -p value              Path to monitor, will not be recursed [$SINGLE_PATH]
   --recursive-path value, --rp value  Path to monitor with recursion [$RECURSIVE_PATH]
   --log-level value, -l value         Log Level (default: "info") [$LOGLEVEL]
   --config value                      configuration file
   --help, -h                          show help (default: false)
   --version, -v                       print the version (default: false)
```
