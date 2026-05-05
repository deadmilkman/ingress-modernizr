
.PHONY: build plugin-layout

build:
	go build -o ./dist/ingress-modernizr main.go 

plugin-layout: build
	mkdir -p ./dist/plugin/helm4
	cp ./plugin/helm4/plugin.yaml ./dist/plugin/helm4/plugin.yaml
	cp ./dist/ingress-modernizr ./dist/plugin/helm4/ingress-modernizr
