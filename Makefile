GO ?= go
export CGO_ENABLED = 1
export PROJ_NETWORK = OFF

BENCH_DIR ?= benchmark
BENCH_DATASETS ?= $(BENCH_DIR)/work/datasets
BENCH_RESULTS ?= $(BENCH_DIR)/results
BENCH_CACHE ?= $(BENCH_DIR)/.cache
VICENTE_REPO ?= https://github.com/vicentecalfo/eoo-aoo-calculator.git
VICENTE_COMMIT ?= d47f41deb6041b622aa9aa1587bff4702b953b73
VICENTE_DIR ?= $(BENCH_CACHE)/vicentecalfo-eoo-aoo-calculator
CONR_COMMIT ?= 50b9924bcd6bcec2bf2f2d775fa47bb29b721ba6
CONR_RLIB ?= $(BENCH_CACHE)/Rlib

.PHONY: build test check fmt doctor clean \
	benchmark benchmark-doctor benchmark-check benchmark-deps-debian \
	benchmark-prepare benchmark-self benchmark-vicente-setup benchmark-vicente \
	benchmark-conr-setup benchmark-conr benchmark-collect benchmark-report \
	benchmark-clean benchmark-clean-all

build:
	mkdir -p bin
	$(GO) build -buildvcs=false -trimpath -o bin/eoo-aoo ./cmd/eoo-aoo

test:
	$(GO) test -race ./...

check:
	@test -z "$$($(GO) fmt ./...)" || (echo 'Go formatting needed'; exit 1)
	$(GO) vet ./...
	$(GO) test -race ./...

fmt:
	$(GO) fmt ./...

doctor:
	$(GO) version
	pkg-config --modversion gdal proj
	$(GO) env CGO_ENABLED

clean:
	rm -rf bin

benchmark-doctor:
	@command -v python3 >/dev/null || (echo 'python3 is required'; exit 1)
	@command -v git >/dev/null || (echo 'git is required'; exit 1)
	@command -v node >/dev/null || (echo 'node is required'; exit 1)
	@command -v npm >/dev/null || (echo 'npm is required'; exit 1)
	@command -v Rscript >/dev/null || (echo 'Rscript is required'; exit 1)
	@echo "python: $$(python3 --version)"
	@echo "node:   $$(node --version)"
	@echo "npm:    $$(npm --version)"
	@echo "R:      $$(Rscript --version 2>&1 | head -n 1)"
	@echo "Vicente reference commit: $(VICENTE_COMMIT)"
	@echo "ConR reference commit:    $(CONR_COMMIT)"

benchmark-check:
	python3 -m py_compile $(BENCH_DIR)/scripts/prepare.py $(BENCH_DIR)/scripts/run_self.py $(BENCH_DIR)/scripts/collect.py
	node --check $(BENCH_DIR)/scripts/run_vicente.mjs
	Rscript -e 'parse(file="$(BENCH_DIR)/scripts/setup_conr.R"); parse(file="$(BENCH_DIR)/scripts/run_conr.R"); cat("R benchmark scripts parse successfully\n")'

benchmark-deps-debian:
	sudo apt-get update
	sudo apt-get install -y python3 git nodejs npm r-base r-base-dev r-cran-lwgeom build-essential gfortran \
		libgdal-dev libproj-dev libgeos-dev libudunits2-dev libcurl4-openssl-dev libssl-dev libxml2-dev

benchmark-prepare:
	python3 $(BENCH_DIR)/scripts/prepare.py

benchmark-self: build benchmark-prepare
	python3 $(BENCH_DIR)/scripts/run_self.py \
		--binary ./bin/eoo-aoo \
		--datasets $(BENCH_DATASETS) \
		--output $(BENCH_RESULTS)/go.csv

benchmark-vicente-setup:
	mkdir -p $(BENCH_CACHE)
	@if [ ! -d "$(VICENTE_DIR)/.git" ]; then \
		git clone $(VICENTE_REPO) $(VICENTE_DIR); \
	fi
	git -C $(VICENTE_DIR) fetch --prune origin
	git -C $(VICENTE_DIR) checkout --detach $(VICENTE_COMMIT)
	@if [ ! -f "$(VICENTE_DIR)/dist/index.js" ] || [ ! -f "$(VICENTE_DIR)/.benchmark-revision" ] || [ "$$(cat $(VICENTE_DIR)/.benchmark-revision 2>/dev/null)" != "$(VICENTE_COMMIT)" ]; then \
		cd $(VICENTE_DIR) && npm ci && npm run build && printf '%s\n' '$(VICENTE_COMMIT)' > .benchmark-revision; \
	else \
		echo "Vicente reference build already cached at $(VICENTE_COMMIT)"; \
	fi

benchmark-vicente: benchmark-prepare benchmark-vicente-setup
	node $(BENCH_DIR)/scripts/run_vicente.mjs \
		$(VICENTE_DIR) $(BENCH_DATASETS) $(BENCH_RESULTS)/vicentecalfo.csv $(VICENTE_COMMIT)

benchmark-conr-setup:
	mkdir -p $(CONR_RLIB)
	Rscript $(BENCH_DIR)/scripts/setup_conr.R $(CONR_COMMIT) $(CONR_RLIB)

benchmark-conr: benchmark-prepare benchmark-conr-setup
	Rscript $(BENCH_DIR)/scripts/run_conr.R \
		$(BENCH_DATASETS) $(BENCH_RESULTS)/conr.csv $(CONR_RLIB) $(CONR_COMMIT)

benchmark-collect:
	python3 $(BENCH_DIR)/scripts/collect.py

benchmark-report: benchmark-collect

benchmark:
	$(MAKE) benchmark-doctor
	$(MAKE) benchmark-check
	$(MAKE) benchmark-prepare
	$(MAKE) benchmark-self
	$(MAKE) benchmark-vicente
	$(MAKE) benchmark-conr
	$(MAKE) benchmark-collect

benchmark-clean:
	rm -rf $(BENCH_DIR)/work $(BENCH_DIR)/.cache $(BENCH_RESULTS)/raw
	rm -f $(BENCH_RESULTS)/go.csv $(BENCH_RESULTS)/vicentecalfo.csv $(BENCH_RESULTS)/conr.csv
	rm -f $(BENCH_RESULTS)/comparison.csv $(BENCH_RESULTS)/comparison.md
	@echo "Preserved $(BENCH_RESULTS)/manual; use make benchmark-clean-all to remove manual measurements too."

benchmark-clean-all:
	rm -rf $(BENCH_DIR)/work $(BENCH_DIR)/results $(BENCH_DIR)/.cache
