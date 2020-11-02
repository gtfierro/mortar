GO_SRC_FILES := $(wildcard internal/*/*.go)
RS_SRC_FILES := $(wildcard reasoner/src/*.rs)
mortar-server: $(GO_SRC_FILES) main.go
	go build -o mortar-server
	cp mortar-server docker/mortar-server/.

reasoner: $(RS_SRC_FILES)
	cd reasoner && cargo build --release
	cp reasoner/target/release/reasoner docker/reasoner/.

run-server: mortar-server
	MORTAR_HTTP_ADDRESS=localhost MORTAR_HTTP_PORT=5001 MORTAR_DB_HOST=localhost MORTAR_DB_PORT=5434 MORTAR_DB_USER=mortarchangeme MORTAR_DB_PASSWORD=mortarpasswordchangeme MORTAR_DB_DATABASE=mortar MORTAR_REASONER_ADDRESS=localhost:3030 ./mortar-server
