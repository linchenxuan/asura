TEST_TAGS = "all"
TEST_ROOT="./test/"
TEST_PATH="${TEST_ROOT}..."

.PHONY: test

ifneq (${TEST_TAGS},"all")
	TEST_PATH="${TEST_ROOT}${TEST_TAGS}/..."
endif

test:
	@go test -v ${TEST_PATH}

