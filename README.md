# raft

## Get all nodes

```http request
curl --request GET \
  --url http://localhost:8080/nodes
```

```
1: 23d898cf-1c1e-449f-9032-e30ffabdc9a5 | Role:Leader | Term:2 | JournalLen: 15 | alive=true
2: 36ea6177-50b7-411c-b2d6-efcd61a0a43a | Role:Leader | Term:1 | JournalLen: 15 | alive=true
3: 87eaf3ab-7f5d-474a-85db-61aa589c073e | Role:Follower | Term:2 | JournalLen: 15 | alive=true
4: bba075a0-240e-4212-901a-b76a984d1be9 | Role:Follower | Term:2 | JournalLen: 15 | alive=true
5: 34bc58bd-3a81-4c1a-b4c1-e9342f4461ac | Role:Follower | Term:2 | JournalLen: 15 | alive=true
```

## Get node journal

```http request
curl --request GET \
  --url 'http://localhost:8080/journal?node=23d898cf-1c1e-449f-9032-e30ffabdc9a5'
```

```
Journal of 23d898cf-1c1e-449f-9032-e30ffabdc9a5:
0:{TERM:0, DATA:"[222 173 190 239]"}
1:{TERM:0, DATA:"map[hello:1]"}
2:{TERM:0, DATA:"map[hello:1]"}
3:{TERM:0, DATA:"map[hello:1]"}
4:{TERM:0, DATA:"map[hello:1]"}
5:{TERM:0, DATA:"map[hello:1]"}
6:{TERM:0, DATA:"map[hello:1]"}
7:{TERM:0, DATA:"map[hello:1]"}
8:{TERM:0, DATA:"map[bog:2]"}
9:{TERM:0, DATA:"map[rfefer:2]"}
10:{TERM:0, DATA:"map[rfefer:2]"}
11:{TERM:0, DATA:"map[rfefer:2]"}
12:{TERM:0, DATA:"map[bye:cat hello:world]"}
```

## Send request to set key:value in distributed storage

```http request
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
{map[key:world value:cat] 23d898cf-1c1e-449f-9032-e30ffabdc9a5}
```

## Kill node
```http request
curl --request GET \
  --url 'http://localhost:8080/kill?node=bba075a0-240e-4212-901a-b76a984d1be9'
```

## Recover node
```http request
curl --request GET \
--url 'http://localhost:8080/recover?node=bba075a0-240e-4212-901a-b76a984d1be9'
```

## Get node storage dump
```http request
curl --request GET \
  --url 'http://localhost:8080/dump?node=23d898cf-1c1e-449f-9032-e30ffabdc9a5'
```

```
Map of 23d898cf-1c1e-449f-9032-e30ffabdc9a5:
map[world:cat]
```

## Get value from storage by key
```http request
curl --request GET \
  --url 'http://localhost:8080/get?node=36ea6177-50b7-411c-b2d6-efcd61a0a43a&key=world'
```

```
cat
```

## Connect nodes
```http request
curl --request GET \
  --url 'http://localhost:8080/connect?node=23d898cf-1c1e-449f-9032-e30ffabdc9a5&with=36ea6177-50b7-411c-b2d6-efcd61a0a43a'
```

```
true
```

## Disconnect nodes
```http request
curl --request GET \
  --url 'http://localhost:8080/disconnect?node=23d898cf-1c1e-449f-9032-e30ffabdc9a5&with=36ea6177-50b7-411c-b2d6-efcd61a0a43a'
```

```
true
```

## Get node topology
```http request
curl --request GET \
  --url 'http://localhost:8080/topology?node=36ea6177-50b7-411c-b2d6-efcd61a0a43a'
```

```
TOPOLOGY FOR 36ea6177-50b7-411c-b2d6-efcd61a0a43a
34bc58bd-3a81-4c1a-b4c1-e9342f4461ac --> true
23d898cf-1c1e-449f-9032-e30ffabdc9a5 --> false
87eaf3ab-7f5d-474a-85db-61aa589c073e --> true
bba075a0-240e-4212-901a-b76a984d1be9 --> true
```