# Fishtail

Fishtail is an AT Protocal event stream listener for habitat. It listens to the event stream from a user's PDS, and determines which Habitat applications are affected by the events. It then fetches the relevant records and sends them to the appropriate applications.

It is very important to understand that Fishtail doesn't just forward the record pertaining to the event, but also all records related to the initial record. For example, if a user likes a post, Fishtail will recursively fetch related records such as the post itself. This is so that the various subscribing Habitat apps don't have to access the PDS themselves to get related records pertaining to the original event.

## Builing Applications with Fishtail

### Configuring Applications
A Habitat app is configured to work with Fishtail by sepcifying reverse proxy rules with the "fisthail_ingest" type. This proxy rule tells Habitat that certain incoming AT Proto events should be forwarded to the Habitat app, at that endpoint. For example, the Pouch app is configured like this:

```yaml
 reverse_proxy_rules:
    - type: redirect
      matcher: /pouch_api
      target: http://host.docker.internal:6000

    - type: fishtail_ingest
      matcher: /pouch_api/ingest
      target: http://host.docker.internal:6000/api/v1/ingest
      fishtail_ingest_config:
        subscribed_collections:
          - lexicon: app.bsky.feed.like
          - lexicon: com.habitat.pouch.link
- app_installation:
    name: pouch_backend
    version: 1
    driver: docker

    driver_config:
      env:
        - PORT=6000
      mounts:
        - type: bind
          source: /home/username/.habitat/apps/pouch/database.sqlite
          target: /app/database.sqlite
      exposed_ports:
        - "6000"
      port_bindings:
        "6000/tcp":
          - HostIp: "0.0.0.0"
            HostPort: "6000"


```
Note that the reverse proxy rule specifies a target endpoint pointing to the Pouch app, with a specific sub-route (`/api/v1/ingest`). This is where Fishtail will forward the events to. Inside the `fishtail_ingest_config`, we specify that only events pertaining to the two collections `app.bsky.feed.like` and `com.habitat.pouch.link` should be forwarded to the Pouch app.

### Handling Events Comfing From Fishtail

Records forwarded to the Habitat app will be sent in a `POST` request with a JSON body containing a list of records.  An example `POST` to the Pouch app is shown below:

```json
{
  "collection": "app.bsky.feed.like",
  "initial_record_uri": "at://did:plc:l3weetptg3ulkbbn7w5zpu2r/app.bsky.feed.like/3lb3eq6ggvk2e",
  "records": [
    {
      "uri": "at://did:plc:l3weetptg3ulkbbn7w5zpu2r/app.bsky.feed.like/3lb3eq6ggvk2e",
      "cid": "bafyreicswpgtdnh4irurekixvlv37w2522cvxt2mu4oreoae3lmafcxvg4",
      "value": {
        "$type": "app.bsky.feed.like",
        "createdAt": "2024-11-16T17:04:12.612Z",
        "subject": {
          "cid": "bafyreiapyw7wgtzbubt77pyjktbtmegwh3hef24ckkgmeturkszdwhtmdq",
          "uri": "at://did:plc:af3pkeo47zrhiyk3tx7jgaf4/app.bsky.feed.post/3layfzdorys2q"
        }
      }
    },
    {
      "uri": "at://did:plc:af3pkeo47zrhiyk3tx7jgaf4/app.bsky.feed.post/3layfzdorys2q",
      "cid": "bafyreiapyw7wgtzbubt77pyjktbtmegwh3hef24ckkgmeturkszdwhtmdq",
      "value": {
        "$type": "app.bsky.feed.post",
        "createdAt": "2024-11-15T12:49:15.222Z",
        "embed": {
          "$type": "app.bsky.embed.images",
          "images": [
            {
              "alt": "Winky the Pirate Cat, one-eyed wonder of the sea lanes, settled on a pillow with a black and white checkerboard case. ",
              "aspectRatio": {
                "height": 2000,
                "width": 1500
              },
              "image": {
                "$type": "blob",
                "mimeType": "image/jpeg",
                "ref": {
                  "$link": "bafkreidt2vjugita3qbj2bcdp2nx7rdmamflalbk3y5m3cmwiq35zyk7ce"
                },
                "size": 637692
              }
            }
          ]
        },
        "langs": [
          "en"
        ],
        "text": "Time to say good night. Good night from me and Winky. Good night."
      }
    }
  ]
}
```
Once this request is received, the app is responsible for figuring out what parts of the data it wants to store, and what parts it can discard.

### Creating New ATProto Records
Sometimes, apps aren't just interested in consuming ATProto records, but also want to create new ones. For example, when someone uses the Pouch browser extension to save a link, we want to add that link to the user's PDS repo under the `com.habitat.pouch.link` collection. 

The way the Pouch backend handles this is to first create a request to the PDS to create the record. From there, Fishtail will see the event, and forward that record back to the Pouch backend. From there, the Pouch backend can parse the record and store it appropriately in it's own SQLite database. While this flow is circular (client -> Pouch Backend -> PDS -> Fishtail -> Pouch Backend again), it allows for the PDS to be the source of truth for all records. The Pouch backend's database is really just a cache of all the data it cares about. In that sense, it plays the same role as an indexer on Bluesky.

## Fishtail Internals

There are a couple key components to Fishtail:
- The `FirehoseConsumer`, which is a function that subscribes to the ATProto event stream from the PDS. It queues up events that need to be ingested.
- The `Ingester`, which takes events from the firehose and ingests them into the local Habitat database. For each record queued up, it will find all records linked to that record, and add them to it's own internal queue, and keep on ingesting records by querying the PDS until the queue is exhausted. Note that the first record ingested doesn't trigger a `GET` on the PDS, but the subsequent records in that chain do.
- The `Publisher`, which takes a chain of records and publishes them to all of the subscribers that are interested in that collection.

## Next Steps
- [ ] Allow for listening to multiple PDSs
- [ ] Max depth of chained records that can be ingested
- [ ] Better batching and queueing of outgoing publish requests
