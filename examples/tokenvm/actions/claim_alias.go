package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*ClaimAlias)(nil)

type ClaimAlias struct {
	Alias []byte `json:"alias"`
}

func (*ClaimAlias) StateKeys(rauth chain.Auth, _ ids.ID) [][]byte {
	return [][]byte{storage.PrefixAliasKey(auth.GetActor(rauth))}
}

func (c *ClaimAlias) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	txID ids.ID,
	_ bool,
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	unitsUsed := c.MaxUnits(r) // max units == units
	if len(c.Alias) > MaxAliasSize {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputAliasTooLarge}, nil
	}

	//TODO: manage alias ownership so claim is not possible if owner(alias)  != actor
	exists, alias, isWarp, err := storage.GetAlias(ctx, db, m.Asset)

	// if !exists, it means that the Alias is available
	if exists {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputAlreadyHasAlias}, nil
	}
	if err := storage.SetAlias(ctx, db, actor, c.Alias, false); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.OwnAlias(ctx, db, actor, c.Alias, false); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}

	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (c *ClaimAlias) MaxUnits(chain.Rules) uint64 {
	// We use size as the price of this transaction but we could just as easily
	// use any other calculation.
	return uint64(len(c.Alias))
}

func (c *ClaimAlias) Marshal(p *codec.Packer) {
	p.PackBytes(c.Alias)
}

func UnmarshalClaimAlias(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var create CreateAlias
	p.UnpackBytes(MaxAliasSize, false, &create.Alias)
	return &create, p.Err()
}

func (*ClaimAlias) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
