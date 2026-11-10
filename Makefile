# Backend targets (backend-*). Other targets are added by the orchestrator.
BACKEND := backend

.PHONY: backend-run backend-seed backend-demo backend-build backend-test backend-race backend-vet backend-fmt backend-check

backend-run:
	cd $(BACKEND) && go run ./cmd/server

backend-seed:
	cd $(BACKEND) && go run ./cmd/server --seed-only

backend-demo:
	cd $(BACKEND) && DEMO_MODE=true go run ./cmd/server --seed

backend-build:
	cd $(BACKEND) && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o bin/modusvoice ./cmd/server

backend-test:
	cd $(BACKEND) && go test ./...

backend-race:
	cd $(BACKEND) && CGO_ENABLED=1 go test -race ./...

backend-vet:
	cd $(BACKEND) && go vet ./...

backend-fmt:
	cd $(BACKEND) && gofmt -l -w .

backend-check: backend-fmt backend-vet backend-test
