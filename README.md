# fsort

Sort file paths by metadata.

## Installation

```shell
go install github.com/CatsDeservePets/fsort@latest
```

## Usage

```
usage: fsort [-f] [-z] [-C dir] [-k key | -K key] ... [file ...]
  -C dir
    	change to dir before resolving input names
  -K key
    	same as -k, but sorts by key in descending order
  -f	fold lowercase characters to uppercase before comparison
  -k key
    	sort by key in ascending order. Key must be one of name,
    	path, extension, type, perm, size, or time. The -k and -K
    	options may be specified multiple times; subsequent keys
    	are compared when earlier keys compare equal. By default,
    	fsort sorts by name.
  -z	line delimiter is NUL, not newline
```

## Examples

Find the most recently modified log files:

```shell
find /var/log -type f -name '*.log' | fsort -K time
```

Find the largest archives in a directory tree:

```shell
find . -type f \( -name '*.zip' -o -name '*.tar.gz' -o -name '*.iso' \) | fsort -K size
```

Sort images by type, then by name:

```shell
fsort -k extension -k name *.jpg *.png *.gif *.webp
```
