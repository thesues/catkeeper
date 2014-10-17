.PHONY: all

all:catkeeper

GOPATH = $(PWD)/build
#export GOPATH


URL = github.com/thesues
REPO = catkeeper

URLPATH = $(GOPATH)/src/$(URL)

catkeeper:web/*.go
		@[ -d $(URLPATH) ] || mkdir -p $(URLPATH)
		@ln -nsf $(PWD) $(URLPATH)/$(REPO)
		go install $(URL)/$(REPO)/web/
