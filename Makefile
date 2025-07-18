build-amd:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static"' -o dist/qmp-controller-amd64

build-arm:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static"' -o dist/qmp-controller-arm64


build: build-arm

scp: build
	scp ./dist/qmp-controller-amd64 pve1:~/qmp-controller
