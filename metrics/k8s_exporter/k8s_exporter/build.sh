#CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o k8s_exporter main.go
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o k8s_exporter main.go
