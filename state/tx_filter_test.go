package state_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/classic/db"
	cmn "github.com/tendermint/classic/libs/common"
	sm "github.com/tendermint/classic/state"
	"github.com/tendermint/classic/types"
)

func TestTxFilter(t *testing.T) {
	genDoc := randomGenesisDoc()
	genDoc.ConsensusParams.Block.MaxTxBytes = 3000

	testCases := []struct {
		tx    types.Tx
		isErr bool
	}{
		{types.Tx(cmn.RandBytes(250)), false},
		{types.Tx(cmn.RandBytes(3000)), false},
		{types.Tx(cmn.RandBytes(3001)), true},
		{types.Tx(cmn.RandBytes(5000)), true},
	}

	for i, tc := range testCases {
		stateDB := dbm.NewDB("state", "memdb", os.TempDir())
		state, err := sm.LoadStateFromDBOrGenesisDoc(stateDB, genDoc)
		require.NoError(t, err)

		f := sm.TxPreCheck(state)
		if tc.isErr {
			assert.NotNil(t, f(tc.tx), "#%v", i)
		} else {
			assert.Nil(t, f(tc.tx), "#%v", i)
		}
	}
}
