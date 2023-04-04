package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*TransferWithAlias)(nil)

type TransferWithAlias struct {
	// AliasTo is the recipient of the [Value].
	AliasTo []bytes `json:"to_alias"`

	// Asset to transfer to [To].
	Asset ids.ID

	// Amount are transferred to [To].
	Value uint64 `json:"value"`
}

func (t *TransferWithAlias) StateKeys(rauth chain.Auth, _ ids.ID) [][]byte {
	return [][]byte{
		exists, to, isWarp, err := storage.GetAddressFromAlias(t.AliasTo)
		storage.PrefixBalanceKey(auth.GetActor(rauth), t.Asset),
		storage.PrefixBalanceKey(to, t.Asset),
	}
}

func (t *TransferWithAlias) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
	_ bool,
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	unitsUsed := t.MaxUnits(r) // max units == units
	if t.Value == 0 {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputValueZero}, nil
	}
	if err := storage.SubBalance(ctx, db, actor, t.Asset, t.Value); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	
	exists, to, isWarp, err := storage.GetAddressFromAlias(t.AliasTo)
	
	if err := storage.AddBalance(ctx, db, to, t.Asset, t.Value); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (*TransferWithAlias) MaxUnits(chain.Rules) uint64 {
	// We use size as the price of this transaction but we could just as easily
	// use any other calculation.
	return crypto.PublicKeyLen + consts.IDLen + consts.Uint64Len
}

func (t *TransferWithAlias) Marshal(p *codec.Packer) {
	exists, to, isWarp, err := storage.GetAddressFromAlias(t.AliasTo)
	p.PackPublicKey(to)
	p.PackID(t.Asset)
	p.PackUint64(t.Value)
}

func UnmarshalTransferWithAlias(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var transfer_with_alias TransferWithAlias
	exists, to, isWarp, err := storage.GetAddressFromAlias(t.AliasTo)
	p.UnpackPublicKey(false, &to) // can transfer to blackhole
	p.UnpackID(false, &transfer_with_alias.Asset)     // empty ID is the native asset
	transfer_with_alias.Value = p.UnpackUint64(true)
	return &transfer_with_alias, p.Err()
}

func (*TransferWithAlias) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
