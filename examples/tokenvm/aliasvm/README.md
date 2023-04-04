# Create a `hypervm` leveraging the existing `tokenvm`

## Introduction

In this tutorial, we add "Alias" functionality to `tokenvm` using `hypersdk`. This new Alias will work similar to a domain name service mapping each address to a human-readable alias, allowing transfers to happen using the alias instead of the public keys. This is important to create better user experience to interact with the blockchain.

---

**WARNING!** `hypersdk` is considered **ALPHA** software and is not safe to use in production. The framework is under active development and may change significantly over the coming months as its modules are optimized and audited.

---

## Requirements

- [Introduction to VMs](https://docs.avax.network/subnets/introduction-to-vm)
- Go v1.20 or later. Follow this guide to [install](https://go.dev/doc/install) Go and confirm the version running `go version`

## Instructions

To start, clone the `hypersdk` repository and access the directory `tokenvm` under examples. This directory contains all the TokenVM-specific functionality.

```sh
git clone https://github.com/ava-labs/hypersdk
cd examples/tokenvm
```

### Storage Functions

To add Alias features you will need to keep track of which alias belongs to which address, in order to do this, we need to create some functions to interact with the database. This functions are managed under the file `storage/storage.go` and will store all data in a _key-value_ form inside the database imported.

_**Storage Prefixes**_
Each _key_ from the data base, needs a Prefix to identify the functionality that is addressed. This prefix will also help when parsing the value from the database.

The intuition here is that every Address (key) will be linked to an specific Alias (value) in the database, but since each address might have several data associated to them, such as balance, a KYC flag, or any other info related to the address , we need to prepend some identifier to specify we are storing Alias data, we will be using a new aliasPrefix. And because we also want each alias to retrieve a unique address associated to it in order to send transactions the public key address associated to the alias, we will need to add a second prefix, "aliasOwnerPrefix". This way we will have 2 types of registries, one with Address as the key, and other with Alias as the key.

The TokenVM already keeps track of 6 prefixes to relate to (balance, asset, order, loan, incomingWarp, outcomingWarp), Notice that not all the prefixes have relation with the address, but will be useful to identify in an unequivocal way every key.

```go
const (
	txPrefix = 0x0

	balancePrefix      = 0x0
	assetPrefix        = 0x1
	orderPrefix        = 0x2
	loanPrefix         = 0x3
	incomingWarpPrefix = 0x4
	outgoingWarpPrefix = 0x5
	aliasPrefix        = 0x6
	aliasOwnerPrefix   = 0x7
)
```

_**Set key**_
Now that we have created our prefixes, lets create a the functions that will help us creating our Prefix-appended keys

```go
// Key will be [aliasPrefix]+[address]
func  PrefixAliasKey(pk crypto.PublicKey) (k []byte) {
	k  =  make([]byte, 1+crypto.PublicKeyLen)
	k[0] = aliasPrefix
	copy(k[1:], pk[:])
	return
}
// Key will be [aliasOwnerPrefix]+[alias]
func  PrefixAliasOwnerKey(alias []byte) (k []byte) {
	k  =  make([]byte, 1+len(alias))
	k[0] = aliasOwnerPrefix
	copy(k[1:], alias)
	return
}
```

_**Set value and Store in Database**_
Now, let's create the functions to create our _value_ and the ones that will in fact store our _key:value_ pairs in the database.

This first function will create the the relation _Address-Alias_. Notice that the value needs to include the length of the alias in the first byte so we can easily parse it when we retrieve the value. The last byte of the value will work as a boolean to include Avalanche Warp Messaging functionality. Remember to always carry the context when interacting with multiple processes, in this case interacting with the database.

```go
// Value will be [len(alias)]+[alias]+[warp]
func  SetAlias(
	ctx context.Context,
	db chain.Database,
	pk crypto.PublicKey,
	alias []byte,
	warp bool,
	) error {
	k  :=  PrefixAliasKey(pk)
	aliasLen  :=  len(alias)
	v  :=  make([]byte, consts.Uint16Len+aliasLen+1)
	binary.BigEndian.PutUint16(v, uint16(aliasLen))
	copy(v[consts.Uint16Len:], alias)
	b  :=  byte(0x0)
	if warp {
	b  =  0x1
	}
	v[consts.Uint16Len+aliasLen] = b
	return db.Insert(ctx, k, v)
}
```

You will need to do the same with the inverse relation Alias to Address with the aliasOwnerPrefix. In this other case, you don't need to store the address length since this can be imported from the constants when needed to retrieve.

_**Retrieve and Parse Value from Data Base**_
Now, lets take a look to how to retrieve function and how to parse the value to extract all the info associated. Notice that when parsing the value with `innerGetAlias` you need to specify the bytes of the value in which each of the elements were stored, here is where having stored the length of the Alias at the first byte of the value becomes useful.

```go
// Get raw value from database
func  GetAlias(ctx context.Context, db chain.Database, pk crypto.PublicKey) (bool, []byte, bool, error) {
	k  :=  PrefixAliasKey(pk)
	v, err  := db.GetValue(ctx, k)
	return  innerGetAlias(v, err)
}

// Parse value and some other values to use as the variables: exists, alias, isWarp, err
func  innerGetAlias(
	v []byte,
	err error,
	) (bool, []byte, bool, error) {
	if errors.Is(err, database.ErrNotFound) {
		return  false, nil, false, nil
	}
	if err !=  nil {
		return  false, nil, false, err
	}
	aliasLen  := binary.BigEndian.Uint16(v)
	alias  := v[consts.Uint16Len : consts.Uint16Len+aliasLen]
	warp  := v[consts.Uint16Len+aliasLen] ==  0x1
	return  true, alias, warp, nil
}
```

For an ease of read of this guide, we left the equivalent parsing for the Public Key Address from the relation Alias:Address as an exercise to be consulted in the repository itself.

_**Remove registry from database**_
Finally, to delete any registry call the `Remove` method on the database key. Again, don't forget to carry the context.

```go
func  DeleteAlias(ctx context.Context, db chain.Database, pk crypto.PublicKey) error {
	k  :=  PrefixAliasKey(pk)
	return db.Remove(ctx, k)
}
```

### Actions

At this point we have told our VM how to store new Aliases in the database. Now we need to create some specific _actions_ to let users to interact with the blockchain runtime. You can think of them as the actions that each transaction need to execute in order to interact in any way with the blockchain. These actions can interact with the storage functions we defined before to modify the database and state of the blockchain depending on the action performed.

To illustrate how actions are created, lets look at the elements in the action Interface:

```go
type Action interface {
	MaxUnits(Rules) uint64
	ValidRange(Rules) (start int64, end int64)

	StateKeys(auth Auth, txID ids.ID) [][]byte
	Execute(
		ctx context.Context,
		r Rules,
		db Database,
		timestamp int64,
		auth Auth,
		txID ids.ID,
		warpVerified bool,
	) (result *Result, err error)

	Marshal(p *codec.Packer)
}
```

Now, lets create a new file where we will define our action `ClaimAlias`, call this new file `actions/claim_alias.go`. This action will create a mapping for the sender address to the human-readable alias.

Additionally to the `ClaimAlias` action described below, you can add more functionality with actions such as:

- `TransferWithAlias`
- `ModifyAlias`
- `RemoveAlias`
  and much more...

_**Transaction**_
But before we fully jump into Actions, it is important to understand that an Action is part of the structure of each transaction processed by every participant in the network. Even though this is something completely managed by `hypersdk`, it will help understanding the data flow of each element inside the vm.

```go
type  Transaction  struct {
	Base *Base `json:"base"`
	WarpMessage *warp.Message `json:"warpMessage"`
	Action Action `json:"action"`
	Auth Auth `json:"auth"`
	digest []byte
	bytes []byte
	size uint64
	id ids.ID
	numWarpSigners int
	warpID ids.ID
}
```

Each action will be defined as a `struct` containing the parameters that the user will need to provide to perform the action.

```go
type  ClaimAlias  struct {
	Alias []byte  `json:"alias"`
}
```

_**Action State Keys**_
Now lets generate the State Keys needed to validate each transactions. As before, this will prepend the Prefix defined before, but this time the address will be taken directly from the `chain.Auth` module included in `hypersdk` .
Think of this as the account who pays for the the transaction, and we also send a blank parameter to perform as the transaction ID.

```go
func (*ClaimAlias) StateKeys(rauth chain.Auth, _ ids.ID) [][]byte {
	return [][]byte{storage.PrefixAliasKey(auth.GetActor(rauth))}
}
```

_**Action Execute**_
Now, knowing how a transaction is structured, whenever a participant need to verify its validity, the processor will execute the action function `Execute` which will return some `Result`.

```go
func (c *ClaimAlias) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	txID ids.ID,
	_ bool,
	) (*chain.Result, error) {
	actor  := auth.GetActor(rauth)
	unitsUsed  := c.MaxUnits(r) // max units == units
	if  len(c.Alias) > MaxAliasSize {
		return  &chain.Result{Success: false, Units: unitsUsed, Output: OutputAliasTooLarge}, nil
	}

	exists, alias, isWarp, err  := storage.GetAlias(ctx, db, m.Asset)
// if exists == false, it means that the Alias is available
	if exists ==  false {
		if  err  := storage.SetAlias(ctx, db, actor, c.Alias, false); err !=  nil{
			return  &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
		if  err  := storage.OwnAlias(ctx, db, actor, c.Alias, false); err !=  nil {
			return  &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
	}
	return  &chain.Result{Success: true, Units: unitsUsed}, nil
}
```

_**Action Fees**_
To prevent from DDOS atacks caused for spamming the network, every action needs to have associated a maximum cost that will take to implement the full action. This might be proportional of the complexity of the action performed, and will be specified in the `MaxUnits` function.

It is considered Maximum because if the action reverts at some point, it is possible that it does not consumes all the units required by the action.

```go
func (c *ClaimAlias) MaxUnits(chain.Rules) uint64 {
// We use size as the price of this transaction but we could just as easily
// use any other calculation.
	return  uint64(len(c.Alias))
}
```

_**Action Codec**_

Marshal and Unmarshal functions add methods to pack/unpack ids, PublicKeys and Signatures in order to be proccessed correctly.

```go
func (c *ClaimAlias) Marshal(p *codec.Packer) {
	p.PackBytes(c.Alias)
}
```

```go
func  UnmarshalClaimAlias(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var  create CreateAlias
	p.UnpackBytes(MaxAliasSize, false, &create.Alias)
	return  &create, p.Err()
}
```

_**Valid Action**_
The value returned by `ValidRange` will be verified when the the `PreExecute` function of the transaction is processed. This prevents from spending resources in invalid actions. Again, this is all handled by `hypersdk`

```go
func (*ClaimAlias) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return  -1, -1
}
```

_**Handling errors**_
To handle error, you can add some error customization in the `actions/outputs.go` file.

### Controller

Controller initializes the data structures utilized by the `hypersdk` and handles both `Accepted` and `Rejected` block callbacks. For a block to be Accepted, every transaction included, must succeed when adding it to the data base. For this last step, we need to include the new actions to `controller/controller.go` to obtain metrics of the successful transactions.

As stated before, each action returns a result to indicate whether the transaction ended in `Success` or not. In case of `Success` we want to

```go
case  *actions.TransferWithAlias:
c.metrics.transferWithAlias.Inc()
case  *actions.ClaimAlias:
c.metrics.claimAlias.Inc()
```

_**Metrics**_

Finally, Metrics are provided by Prometheus, so in order to keep track of new metrics for each of the alias actions, we need to include the actions in the metrics struct:

```go
claimAlias prometheus.Counter
transferWithAlias prometheus.Counter
```

And include them in the `newMetrics` function.

```go
transferWithAlias: prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: "actions",
	Name: "transfer_with_alias",
	Help: "number of transfer with alias actions",
	}),

claimAlias: prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: "actions",
	Name: "claim_alias",
	Help: "number of claim alias actions",
	}),
```

### Genesis

For this exercise the Genesis file was not modified, but you should definitely consider modifying it if you want to change any default configuration, such as the rules, base fees or initial balances.

## Wrapping up

We have seen how to create a `hypervm` leveraging the existing `tokenvm` by adding Alias capabilities. You now know how to create new `Actions`, how to create and retrieve registries interacting with the database through the `Storage` functions, finally you can integrate useful `Metrics` each time the `Controller` Accepts a Successful transaction.
