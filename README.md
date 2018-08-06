# Hashex

`hashex` is an example server of an asynchronous hashing service.

The overall structure of the code is broken into 4 parts:

  1. task.Manager provides the business logic of running async tasks tracked
     by id.
  2. HashApi layers the desired HTTP API semantics onto the task.Manager,
     and HashTask provides the actual hash operation.
  3. EndPointStatsTracker implements the performance tracking, wrapping the
     HashApi endpoint.
  4. main() plugs everything together and handles shutdown.

The code can be run using

```
go build . && ./hashex -port 8080
```
