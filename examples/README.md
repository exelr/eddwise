# Examples

In order to make examples running you have to install `edd` command with 

```shell
go install github.com/exelr/eddwise/cmd/edd
```

Then you can visit any example directory from terminal and type:

```shell
edd design gen
```

to generate code file for both server and client.

Then to start the server of any example, ie: "pingpong", run:
```shell
go run pingpong/cmd/pingpong
```

or to any project automatically:

```shell
project=$(basename $(pwd)) go run ${project}/cmd/${project}
```

Then you can open in a webserver the `web/pingpong/app.html` or make sure
to edit right the references to the [eddclient.js](../eddclient.js) and [channel.js](pingpong/gen/pingpong/channel.js) files and open it directly from a browser. (see line 10-11 of [app.html](pingpong/web/pingpong/app.html))
