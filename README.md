# File Exporter

Prometheus Exporter for Files.

Inspired by [filestat_exporter](https://github.com/michael-doubez/filestat_exporter).

This project is a little more simplistic and is configurable entirely from the command line, it also monitors for filesystem events and updates the metrics using a library.

It exposes file modified time and the CRC32 hash of the file because it can be represented in float64 which then can be used as a value to a gauge metric in prometheus directly.

## Usage

```bash
file_exporter --file path/to/a/file/or/directory
```
