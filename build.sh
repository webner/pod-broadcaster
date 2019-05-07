#/bin/bash

go build -ldflags "-X main.Version=1.0 -X \"main.BuildDate=$(date)\""
