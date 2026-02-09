//go:build deps
// +build deps

// This file is used to track dependencies for go mod.
// It is not compiled into the binary.
package blockberry

import (
	_ "github.com/BurntSushi/toml"
	_ "github.com/blockberries/cramberry/pkg/cramberry"
	_ "github.com/blockberries/glueberry"
	_ "github.com/blockberries/avlberry"
	_ "github.com/stretchr/testify/require"
	_ "github.com/syndtr/goleveldb/leveldb"
)
