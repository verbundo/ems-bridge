# ems-bridge

Routes messages between endpoints — local filesystem, HTTP, and TIBCO EMS queues/topics — based on a YAML config.

## Requirements

- Go 1.25+
- TIBCO EMS client libraries (`tibems.dll`) on Windows when using JMS components

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

The main `config.yml` sets server-level options and references one or more app files:

```yaml
ip: 0.0.0.0
port: 5556
logLevel: debug
tlsCert: ./certs/cert.pem
tlsKey:  ./certs/key.pem

apps:
  - name: app1
    file: ./app.yml
```

Each app file defines **components** (named connections) and **routes** (message flows).

---

### Components

Components are named, reusable connections.

#### JMS (TIBCO EMS)

```yaml
components:
  - name: ems-dev
    type: jms
    properties:
      provider: tibems
      url: tcp://localhost:7222
      username: admin
      password: ""          # plain text or encr:<key>:<hex> (see Encrypted passwords)
```

---

### Routes

A route wires one or more **starters** (message sources) to one or more **processors** (message transformations / sinks) via **links**.

```yaml
routes:
  - name: my-route
    starters:
      - id: <starter-id>
        type: <starter-type>
        properties: { ... }
    processors:
      - id: <processor-id>
        type: <processor-type>
        properties: { ... }
    links:
      - from: <processor-id>
        to:   <next-processor-id>
```

---

### Starters

#### `file_event` — watch a folder for new files

```yaml
- id: folder-watcher
  type: file_event
  properties:
    inputFolder: ./data/in
    outputFolder: ./data/out          # processed files are moved here
    eventType: FILE_CREATE
    processExistingOnStartup: true
    checkSubfolders: true
    filenameSuffixes: "['txt']"
    filenamePrefixes: "['order']"
```

#### `rest` — expose an HTTP endpoint

```yaml
- id: http-receiver
  type: rest
  properties:
    method: POST
    uri: /inbound
```

#### `jms_queue_consumer` — consume messages from a TIBCO EMS queue

```yaml
- id: ems-consumer
  type: jms_queue_consumer
  properties:
    component-ref: ems-dev            # name of a jms component
    queueName: inbound.q
    acknowledgementMode: AUTO         # AUTO (default) | CLIENT | DUPS_OK
    messageSelector: "OrderType = 'NEW'"   # optional JMS selector
    consumerCount: '2'                # number of concurrent consumers (default: 1)
    jmsProperties:                    # optional: enrich message properties from incoming JMS headers
      - name: order.type
        value: Properties["OrderType"]
      - name: filename
        value: Headers["filename"]
        condition: 'Headers["filename"] != ""'
```

**Notes:**
- `acknowledgementMode: CLIENT` acknowledges each message only after the route completes successfully.
- If the incoming message carries a `JMSReplyTo` header, the consumer automatically sends the route's output payload back to that destination after route execution.
- `jmsProperties` expressions are evaluated against `Payload`, `Headers`, and `Properties` of the received message.

---

### Processors

#### `transform` — transform the message payload using an expression

```yaml
- id: transform-payload
  type: transform
  properties:
    type: custom
    script: |
      'Hello, ' + string(Payload)
```

#### `jms_send` — send a message to a TIBCO EMS queue or topic

```yaml
- id: ems-sender
  type: jms_send
  properties:
    component-ref: ems-dev            # name of a jms component
    destination-type: queue           # queue | topic
    destination: outbound.q

    # Quality of Service (all optional)
    deliveryMode: PERSISTENT          # PERSISTENT (default) | NON_PERSISTENT
    priority: 4                       # 0–9, default 4
    expiration: 30000                 # TTL in ms; omit or 0 = no expiration

    # Request-reply (all optional)
    expectReply: "true"               # wait for a reply message (default: false)
    useTmpReplyDestination: "true"    # create a temporary reply queue automatically
    replyDestination: '"reply.q"'     # expression → reply queue name (alternative to tmp)
    replyTimeout: '5000'              # ms to wait for reply; 0 = wait forever

    # JMS message properties (evaluated as expressions at send time)
    jmsProperties:
      - name: OrderType
        value: Properties["order.type"]
      - name: FileName
        value: Headers["filename"]
        condition: 'Headers["filename"] != ""'
```

**Notes:**
- `jmsProperties` are evaluated against `Payload`, `Headers`, and `Properties` of the current message.
- After a successful send, `jms.message.id` is set on the message for use by downstream processors.
- When `expectReply` is true and a reply is received, the message payload is replaced with the reply body.

---

### `jmsProperties` expression environment

Both the `jms_queue_consumer` starter and `jms_send` processor support a `jmsProperties` list.
Each entry is evaluated at runtime with access to:

| Variable | Type | Description |
|----------|------|-------------|
| `Payload` | `any` | Current message payload |
| `Headers` | `map[string]string` | Message headers (e.g. `filename`) |
| `Properties` | `map[string]any` | Message properties |

---

### Full app file example

```yaml
components:
  - name: ems-dev
    type: jms
    properties:
      provider: tibems
      url: tcp://localhost:7222
      username: admin
      password: ""

routes:

  # Consume from EMS, transform, send reply
  - name: ems-request-reply
    starters:
      - id: ems-consumer
        type: jms_queue_consumer
        properties:
          component-ref: ems-dev
          queueName: request.q
          acknowledgementMode: CLIENT
          consumerCount: '3'
          jmsProperties:
            - name: order.type
              value: Properties["OrderType"]
    processors:
      - id: transform-payload
        type: transform
        properties:
          type: custom
          script: '"ACK:" + string(Payload)'
    links: []

  # Watch a folder, send file contents to EMS queue
  - name: folder-to-ems
    starters:
      - id: folder-watcher
        type: file_event
        properties:
          inputFolder: ./data/in
          outputFolder: ./data/out
          eventType: FILE_CREATE
          processExistingOnStartup: true
          filenameSuffixes: "['xml']"
    processors:
      - id: ems-sender
        type: jms_send
        properties:
          component-ref: ems-dev
          destination-type: queue
          destination: outbound.q
          deliveryMode: PERSISTENT
          priority: 4
          jmsProperties:
            - name: FileName
              value: Headers["filename"]
    links: []
```

---

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

Encrypted values are automatically decrypted at startup when loading the app file.

## Runtime data

On first run, `config.db` (SQLite) is created automatically with a `keys` table.
A single 64-character random AES-256 encryption key is seeded as `id=1`.

## Packages

| Package | Description |
|---------|-------------|
| `components` | Named connection pools (JMS/TIBCO EMS) |
| `starters` | Message sources: `file_event`, `rest`, `jms_queue_consumer` |
| `processors` | Message processors: `transform`, `jms_send` |
| `routes` | Route wiring: links starters → processors |
| `encr` | AES-256-GCM encrypt/decrypt tied to keys in `config.db` |
| `sqlite` | SQLite DB setup and key seeding |
| `utils/encr` | CLI tool to encrypt strings for use in app files |
