package client

import (
	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/classic/types"
)

var cdc = amino.NewCodec()

func init() {
	types.RegisterEvidences(cdc)
}
