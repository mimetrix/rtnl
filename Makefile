.PHONY: all
all: build/nl

PKGSRC = addr.go bridge.go errors.go link.go link_test.go loopback.go macvlan.go neighbor.go route.go rtnetlink.go rule.go spec.go tuntap.go util.go veth.go vrf.go vxlan.go


VERSION = $(shell git describe --always --long --dirty)
LDFLAGS = "-X gitlab.com/mergetb/tech/rtnl.Version=$(VERSION)"

build/nl: cmd/nl/main.go cmd/nl/* $(PKGSRC)
	$(QUIET) $(call go-build)

BLUE=\e[34m
GREEN=\e[32m
CYAN=\e[36m
NORMAL=\e[39m

QUIET=@
ifeq ($(V),1)
	QUIET=
endif

define build-slug
	@echo "$(BLUE)$1$(GREEN)\t $< $(CYAN)$@$(NORMAL)"
endef

define go-build
	$(call build-slug,go)
	$(QUIET) \
		go build -ldflags=${LDFLAGS} -o $@ $(dir $<)*.go
endef

clean:
	rm -rf build
