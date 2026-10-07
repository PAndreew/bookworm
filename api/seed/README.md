# Seed corpus

Offline sample texts so the stack can be run and demonstrated without network
access. Each file is plain UTF-8 text of a work in the public domain.

To ingest real books from Project Gutenberg once you have internet access:

    go run ./cmd/ingest -seed                  # the curated 10-book shelf
    go run ./cmd/ingest -gutenberg 1342        # one book by id
    go run ./cmd/ingest -file seed/x.txt \
        -slug x -title "X" -author "Y"         # a local file

The `-file` path is also how publisher uploads are tested before the portal exists.
