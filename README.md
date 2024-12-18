# raft

Коллекция с запросами находится в директории bruno-raft-collection
https://www.usebruno.com/

## Get all nodes

```
curl --request GET \
  --url http://localhost:8080/nodes
```

```
{
  "nodes": [
    {
      "id": "d29b55df-9e93-4dcd-a83b-e6f24b3d6626",
      "role": "Follower",
      "term": 0,
      "journal_len": 1,
      "alive": true
    },
    {
      "id": "fe7320bc-345f-4081-8642-5163da7cdc19",
      "role": "Follower",
      "term": 0,
      "journal_len": 1,
      "alive": true
    },
    {
      "id": "df416274-bb5a-4d2a-b5c0-f734b503812e",
      "role": "Follower",
      "term": 0,
      "journal_len": 1,
      "alive": true
    },
    {
      "id": "ff1b64fc-1db6-4567-9789-b49af98e1625",
      "role": "Follower",
      "term": 0,
      "journal_len": 1,
      "alive": true
    },
    {
      "id": "686331b3-cfe9-4878-b798-6e6465189f81",
      "role": "Leader",
      "term": 0,
      "journal_len": 1,
      "alive": true
    }
  ]
}
```

## Get node journal

```
curl --request GET \
  --url 'http://localhost:8080/journal?node=23d898cf-1c1e-449f-9032-e30ffabdc9a5'
```

```
{
  "id": "784923f2-7472-43d2-a2a4-a807f1e96ed4",
  "log": [
    "0:{TERM:-1, DATA:\"[222 173 190 239]\"}"
  ]
}
```

## Send request to set key:value in distributed storage

```
curl --request GET \
  --url http://localhost:8080/request \
  --header 'content-type: application/json' \
  --data '{
  "id": "23d898cf-1c1e-449f-9032-e30ffabdc9a5",
  "msg": {
    "key": "world",
    "value": "cat"
  }
}'
```

```
{
  "id": "fec11053-437f-4759-9821-31753f9da2a9",
  "key": "world",
  "value": "world"
}
```

## Kill node
```
curl --request GET \
  --url 'http://localhost:8080/kill?node=bba075a0-240e-4212-901a-b76a984d1be9'
```

## Recover node
```
curl --request GET \
--url 'http://localhost:8080/recover?node=bba075a0-240e-4212-901a-b76a984d1be9'
```

## Get node storage dump
```
curl --request GET \
  --url 'http://localhost:8080/dump?node=23d898cf-1c1e-449f-9032-e30ffabdc9a5'
```

```
{
  "id": "604485ad-1712-41c3-9aad-ae45a518a4bb",
  "dump": "map[world:cat]"
}
```

## Get value from storage by key
```
curl --request GET \
  --url 'http://localhost:8080/get?node=36ea6177-50b7-411c-b2d6-efcd61a0a43a&key=world'
```

```
{
  "id": "5cc728af-dea9-412f-8f0c-0af5d27992a5",
  "key": "world",
  "value": "cat"
}
```

## Connect nodes
```
curl --request GET \
  --url 'http://localhost:8080/connect?node=23d898cf-1c1e-449f-9032-e30ffabdc9a5&with=36ea6177-50b7-411c-b2d6-efcd61a0a43a'
```

```
{
  "node": "971f456c-e905-4685-9f61-8189af22f047",
  "with": "4aad792b-fe0a-43a2-bb26-9bf11cb4ab80",
  "status": true
}
```

## Disconnect nodes
```
curl --request GET \
  --url 'http://localhost:8080/disconnect?node=23d898cf-1c1e-449f-9032-e30ffabdc9a5&with=36ea6177-50b7-411c-b2d6-efcd61a0a43a'
```

```
{
  "node": "0b3c0122-0b3b-4ebe-8788-50415cdd0ef7",
  "with": "f9a871b1-f471-4b14-9e00-38c762bb7461",
  "status": true
}
```

## Get node topology
```
curl --request GET \
  --url 'http://localhost:8080/topology?node=36ea6177-50b7-411c-b2d6-efcd61a0a43a'
```

```
{
  "id": "2f3ecd83-ed57-498c-906f-aa54454997e0",
  "nodes": [
    {
      "node": "de65366e-38b1-48b7-ae71-16be1a567adb",
      "connected": true
    },
    {
      "node": "26065218-de14-4308-a034-be93a4fbe607",
      "connected": true
    },
    {
      "node": "b0d6d818-0fe2-49e1-b661-f938aba3c5e1",
      "connected": true
    },
    {
      "node": "d8d26056-1080-43b3-8d82-97703541b3d1",
      "connected": true
    }
  ]
}
```