SRC_FILES := $(wildcard internal/*/*.go)
mortar-server: $(SRC_FILES) main.go
	go build -o mortar-server
	cp mortar-server docker/mortar-server/.

run-server: mortar-server
	MORTAR_HTTP_ADDRESS=localhost MORTAR_HTTP_PORT=5001 MORTAR_DB_HOST=localhost MORTAR_DB_PORT=5434 MORTAR_DB_USER=mortarchangeme MORTAR_DB_PASSWORD=mortarpasswordchangeme MORTAR_DB_DATABASE=mortar ./mortar-server
