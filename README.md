# ems-bridge

Routes messages between endpoints — local filesystem and EMS queues — based on a YAML config.

## Requirements

- Go 1.25+

## Building from source

Clone the repository and fetch dependencies:

```bash
git clone <repo-url>
cd ems-bridge
go mod download
```

Build the main binary and utilities:

```bash
make build        # produces ./ems-bridge
make build-utils  # produces ./utils/encr
```

Or build everything at once:

```bash
make build build-utils
```

Other targets:

```bash
make run     # go run . (requires -c flag, e.g. go run . -c config.yml)
make test    # go test ./...
make clean   # remove binaries
```

## Usage

```bash
./ems-bridge --config config.yml
# or
./ems-bridge -c config.yml
```

## Configuration

Edit `config.yml` to define connectors and routes.

### Connectors

Named endpoints referenced in routes:

```yaml
ems-dev:
  type: ems
  url: "tcp://tibco-dev:11200"
  username: "admin"
  password: "admin"
```

### Routes

Each route has a `from` source and one or more `to` destinations:

```yaml
routes:
  - name: folder-to-ems
    from: "fs:./data/in"
    to:
      - "ems-dev:queue:tmp.q"
      - "fs:./data/out"
```

### Address format

| Prefix | Example | Description |
|--------|---------|-------------|
| `fs:` | `fs:./data/in` | Local filesystem path |
| `<connector>:queue:` | `ems-dev:queue:tmp.q` | Named connector + queue |

### Encrypted passwords

Passwords are encrypted with AES-256-GCM and stored as:

```
encr:<key-prefix>:<hex-encoded-nonce+ciphertext>
```

To encrypt a password, use the `encr` utility. It creates `config.db` and seeds a key automatically if they do not exist:

```bash
./utils/encr "mypassword"
# encr:f8e775de:...
```

Encrypted values are automatically decrypted at startup when loading `config.yml`.

## Runtime data

On first run, `config.db` (SQLite) is created automatically with a `keys` table.
A single 64-character random AES-256 encryption key is seeded as `id=1`.

## Packages

| Package | Description |
|---------|-------------|
| `encr` | AES-256-GCM encrypt/decrypt tied to keys in `config.db` |
| `sqlite` | SQLite DB setup and key seeding |
| `utils/encr` | CLI tool to encrypt strings for use in `config.yml` |
