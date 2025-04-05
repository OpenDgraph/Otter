
Notes:

Investigate http://0.0.0.0:8080/debug/jemalloc

If we can use it as instropection of resources and make a better loadbalance based on it.
Also some CPU, Goroutines could help.

----

This can be used for complex routing and find who is the leader.
Also for balancing tablets

http://0.0.0.0:8080/state

Also, zero :6080/state

```JSON
{
  "counter": "58",
  "groups": {
    "1": {
      "members": {
        "2": {
          "id": "2",
          "groupId": 1,
          "addr": "localhost:7080",
          "leader": true,
          "amDead": false,
          "lastUpdate": "1743741646",
          "learner": false,
          "clusterInfoOnly": false,
          "forceGroupId": false
        }
      },
      "tablets": {
        "0-name": {
          "groupId": 1,
          "predicate": "0-name",
          "force": false,
          "onDiskBytes": "0",
          "remove": false,
          "readOnly": false,
          "moveTs": "0",
          "uncompressedBytes": "0"
        }
      },
      "snapshotTs": "4",
      "checksum": "9827531435607164753",
      "checkpointTs": "0"
    }
  },
  "zeros": {
    "1": {
      "id": "1",
      "groupId": 0,
      "addr": "localhost:5080",
      "leader": true,
      "amDead": false,
      "lastUpdate": "0",
      "learner": false,
      "clusterInfoOnly": false,
      "forceGroupId": false
    }
  },
  "maxUID": "10000",
  "maxTxnTs": "10000",
  "maxNsID": "0",
  "maxRaftId": "2",
  "removed": [],
  "cid": "fe2fd2d5-484c-4e9f-b2b1-194600e534ef",
  "license": {
    "user": "",
    "maxNodes": "18446744073709551615",
    "expiryTs": "1746320854",
    "enabled": true
  }
}
```

----

http://0.0.0.0:8080/health

```JSON
[
  {
    "instance": "alpha",
    "address": "localhost:7080",
    "status": "healthy",
    "group": "1",
    "version": "dev",
    "uptime": 1094,
    "lastEcho": 1743742379,
    "ongoing": [
      "opRollup"
    ],
    "ee_features": [
      "backup_restore",
      "cdc"
    ],
    "max_assigned": 103
  }
]
```

---

:6080/assign

we can use to lease UIDs

and

:6080/moveTablet to balance tablets/data between groups.



## Modes

### Vanilla  
A standard DQL approach with no prefixes—just the basics.
In this mode, you’ll only be using features like the load balancer and its modes. You’ll also be able to analyze queries, use APIs like WebSocket, and get real-time cluster status updates.

### GraphQL Vanilla  
Just like Vanilla but uses Dgraph’s native GraphQL schema modeling for DQL queries.

### DQL Ontology-Like 

A graph model inspired by ontology structure, using prefixes at the predicate level. This makes sharding at the predicate level possible.
This is a fully opinionated approach based on my own design choices.