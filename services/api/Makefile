TYPST_VERSION ?= 0.14.2
DIST := dist

.PHONY: test build lambda layer clean fonts

test:
	go test ./...

build:
	go build ./...

## lambda — compile le binaire pour AWS Lambda (arm64, runtime provided.al2023).
## Le fichier doit s'appeler `bootstrap` : c'est ce que le runtime exécute.
lambda:
	mkdir -p $(DIST)/lambda
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" -o $(DIST)/lambda/bootstrap ./cmd/api

## layer — prépare le layer Lambda : binaire Typst + polices Geist.
## Typst est monté en lecture seule sous /opt plutôt qu'extrait dans /tmp à
## chaque invocation ; les polices suivent le même chemin.
layer:
	mkdir -p $(DIST)/layer/bin $(DIST)/layer/fonts
	curl -fsSL "https://github.com/typst/typst/releases/download/v$(TYPST_VERSION)/typst-aarch64-unknown-linux-musl.tar.xz" \
		-o $(DIST)/typst.tar.xz
	tar -xf $(DIST)/typst.tar.xz -C $(DIST)
	mv $(DIST)/typst-aarch64-unknown-linux-musl/typst $(DIST)/layer/bin/typst
	chmod +x $(DIST)/layer/bin/typst
	cp internal/platform/doc/fonts/*.ttf $(DIST)/layer/fonts/
	rm -rf $(DIST)/typst.tar.xz $(DIST)/typst-aarch64-unknown-linux-musl

## fonts — resynchronise les polices depuis le paquet npm geist, pour que
## l'application et les PDF partagent exactement les mêmes fichiers.
fonts:
	@src=$$(find ../../node_modules/.pnpm -path "*/geist/dist/fonts/geist-sans" -type d | head -1); \
	mono=$$(find ../../node_modules/.pnpm -path "*/geist/dist/fonts/geist-mono" -type d | head -1); \
	for weight in Regular Medium SemiBold Bold; do \
		cp "$$src/Geist-$$weight.ttf" internal/platform/doc/fonts/; \
		cp "$$mono/GeistMono-$$weight.ttf" internal/platform/doc/fonts/; \
	done; \
	echo "polices resynchronisées"

clean:
	rm -rf $(DIST)
