# fsort

Sort file paths by metadata.

## Installation

```shell
go install github.com/CatsDeservePets/fsort@latest
```

## Usage

```
usage: fsort [-a key | -d key]... [file ...]
  -a key
        ascending sort key
  -d key
        descending sort key

The key argument may be name, extension, size, or time.
If no key is specified, name is used in ascending order.
Multiple keys are applied in the order specified.
```

## Examples

Find the most recently modified log files:

```shell
find /var/log -type f -name '*.log' | fsort -d time
```

Find the largest archives in a directory tree:

```shell
find . -type f \( -name '*.zip' -o -name '*.tar.gz' -o -name '*.iso' \) | fsort -d size
```

Sort images by type, then by name:

```shell
fsort -a extension -a name *.jpg *.png *.gif *.webp
```
