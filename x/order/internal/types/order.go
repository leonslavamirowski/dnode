package types

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/olekukonko/tablewriter"

	dnTypes "github.com/dfinance/dnode/helpers/types"
	"github.com/dfinance/dnode/x/market"
)

// Market order object type.
type Order struct {
	// Order unique ID
	ID dnTypes.ID `json:"id"`
	// Order owner account address
	Owner sdk.AccAddress `json:"owner"`
	// Market order belong to
	Market market.MarketExtended `json:"market"`
	// Order type (bid/ask)
	Direction Direction `json:"direction"`
	// Order target price (in quote asset denom)
	Price sdk.Uint `json:"price"`
	// Order target quantity
	Quantity sdk.Uint `json:"quantity"`
	// TimeToLive order auto-cancel period
	Ttl time.Duration `json:"ttl_dur"`
	// Created timestamp
	CreatedAt time.Time `json:"created_at"`
	// Updated timestamp
	UpdatedAt time.Time `json:"updated_at"`
}

// LockCoin return Coin that should be locked (transferred from account to the module).
// Coin denom and quantity are Marked and Order type specific.
func (o Order) LockCoin() (retCoin sdk.Coin, retErr error) {
	coinDenom, coinQuantity := "", sdk.Int{}

	switch o.Direction {
	case Bid:
		quantity, err := o.Market.BaseToQuoteQuantity(o.Price, o.Quantity)
		if err != nil {
			retErr = err
			return
		}
		coinDenom, coinQuantity = o.Market.QuoteDenom(), sdk.NewIntFromBigInt(quantity.BigInt())
	case Ask:
		coinDenom, coinQuantity = o.Market.BaseDenom(), sdk.NewIntFromBigInt(o.Quantity.BigInt())
	default:
		retErr = sdkErrors.Wrap(ErrWrongDirection, o.Direction.String())
		return
	}

	retCoin = sdk.NewCoin(coinDenom, coinQuantity)

	return
}

// Strings returns multi-line text object representation.
func (o Order) String() string {
	b := strings.Builder{}
	b.WriteString("Order:\n")
	b.WriteString(fmt.Sprintf("  ID:        %s\n", o.ID.String()))
	b.WriteString(fmt.Sprintf("  Owner:     %s\n", o.Owner.String()))
	b.WriteString(fmt.Sprintf("  Direction: %s\n", o.Direction.String()))
	b.WriteString(fmt.Sprintf("  Price:     %s\n", o.Price.String()))
	if o.Direction == Bid {
		b.WriteString(fmt.Sprintf("  QQuantity: %s\n", o.Market.QuoteCurrency.UintToDec(o.Quantity).String()))
	} else {
		b.WriteString(fmt.Sprintf("  BQuantity: %s\n", o.Market.BaseCurrency.UintToDec(o.Quantity).String()))
	}
	b.WriteString(fmt.Sprintf("  Ttl:       %s\n", o.Ttl.String()))
	b.WriteString(fmt.Sprintf("  CreatedAt: %s\n", o.CreatedAt.String()))
	b.WriteString(fmt.Sprintf("  UpdatedAt: %s\n", o.UpdatedAt.String()))
	b.WriteString(o.Market.String())

	return b.String()
}

// TableHeaders returns table headers for multi-line text table object representation.
func (o Order) TableHeaders() []string {
	h := []string{
		"O.ID",
		"O.Owner",
		"O.Direction",
		"O.Price",
		"O.QBQuantity",
		"O.TTL",
		"O.CreatedAt",
		"O.UpdatedAt",
	}

	return append(h, o.Market.TableHeaders()...)
}

// TableHeaders returns table rows for multi-line text table object representation.
func (o Order) TableValues() []string {
	v := []string{
		o.ID.String(),
		o.Owner.String(),
		o.Direction.String(),
		o.Price.String(),
	}
	if o.Direction == Bid {
		v = append(v, o.Market.QuoteCurrency.UintToDec(o.Quantity).String())
	} else {
		v = append(v, o.Market.BaseCurrency.UintToDec(o.Quantity).String())
	}
	v = append(v, o.Ttl.String())
	v = append(v, o.CreatedAt.String())
	v = append(v, o.UpdatedAt.String())

	return append(v, o.Market.TableValues()...)
}

// NewOrder creates a new order object.
func NewOrder(
	ctx sdk.Context,
	id dnTypes.ID,
	owner sdk.AccAddress,
	market market.MarketExtended,
	direction Direction,
	price sdk.Uint,
	quantity sdk.Uint,
	ttlInSec uint64) Order {

	return Order{
		ID:        id,
		Owner:     owner,
		Market:    market,
		Direction: direction,
		Price:     price,
		Quantity:  quantity,
		Ttl:       time.Duration(ttlInSec) * time.Second,
		CreatedAt: ctx.BlockTime(),
		UpdatedAt: ctx.BlockTime(),
	}
}

// Order slice type.
type Orders []Order

// Strings returns multi-line text object representation.
func (l Orders) String() string {
	var buf bytes.Buffer

	t := tablewriter.NewWriter(&buf)
	t.SetHeader(Order{}.TableHeaders())

	for _, o := range l {
		t.Append(o.TableValues())
	}
	t.Render()

	return string(buf.Bytes())
}