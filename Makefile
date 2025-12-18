BINARY_NAME=oci-arm-provisioner
VERSION=0.1.0
BUILD_FLAGS=-ldflags="-s -w"

.PHONY: all build clean test run docker install uninstall check-env

all: test build

check-env:
	@if [ -z "$$OCI_ARM_CONFIG" ]; then echo "Environment okay."; fi

build:
	@echo "Building $(BINARY_NAME)..."
	go mod tidy
	go build $(BUILD_FLAGS) -o $(BINARY_NAME) .

test:
	@echo "Running Tests..."
	go test ./... -v
	go vet ./...

clean:
	@echo "Cleaning..."
	go clean
	rm -f $(BINARY_NAME)
	rm -f logs/*.log

run: build
	./$(BINARY_NAME)

# Docker
docker-up:
	docker-compose up -d --build

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Systemd Install (User Mode)
install: build
	@echo "Installing Systemd Service (User Mode)..."
	mkdir -p $(HOME)/.config/oci-arm-provisioner
	[ -f $(HOME)/.config/oci-arm-provisioner/config.yaml ] || cp config.yaml.example $(HOME)/.config/oci-arm-provisioner/config.yaml
	
	mkdir -p $(HOME)/.local/bin
	cp $(BINARY_NAME) $(HOME)/.local/bin/
	
	mkdir -p $(HOME)/.config/systemd/user
	cp deployments/systemd/oci-arm-provisioner.service $(HOME)/.config/systemd/user/
	
	# Fix ExecStart path in service file for local bin
	sed -i 's|/usr/bin/|$(HOME)/.local/bin/|g' $(HOME)/.config/systemd/user/oci-arm-provisioner.service
	
	systemctl --user daemon-reload
	systemctl --user enable --now oci-arm-provisioner
	@echo "Installation Complete. Check logs with: make log"

uninstall:
	systemctl --user disable --now oci-arm-provisioner
	rm -f $(HOME)/.config/systemd/user/oci-arm-provisioner.service
	rm -f $(HOME)/.local/bin/$(BINARY_NAME)
	@echo "Uninstalled."

log:
	tail -f logs/provisioner.log
