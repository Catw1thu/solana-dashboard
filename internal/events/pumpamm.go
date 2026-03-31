package events

type PumpAmmSwapPayload struct {
	Side                   string                     `json:"side"`
	IxName                 string                     `json:"ix_name"`
	Pool                   string                     `json:"pool"`
	User                   string                     `json:"user"`
	BaseMint               string                     `json:"base_mint"`
	QuoteMint              string                     `json:"quote_mint"`
	CoinCreator            string                     `json:"coin_creator"`
	BaseAmountIn           *string                    `json:"base_amount_in"`
	BaseAmountOut          *string                    `json:"base_amount_out"`
	QuoteAmountIn          *string                    `json:"quote_amount_in"`
	QuoteAmountOut         *string                    `json:"quote_amount_out"`
	LpFee                  string                     `json:"lp_fee"`
	ProtocolFee            string                     `json:"protocol_fee"`
	CoinCreatorFee         string                     `json:"coin_creator_fee"`
	Cashback               string                     `json:"cashback"`
	PoolBaseTokenReserves  string                     `json:"pool_base_token_reserves"`
	PoolQuoteTokenReserves string                     `json:"pool_quote_token_reserves"`
	InstructionArgs        PumpAmmSwapInstructionArgs `json:"instruction_args"`
}

type PumpAmmSwapInstructionArgs struct {
	BaseAmountIn      *string `json:"base_amount_in"`
	MinQuoteAmountOut *string `json:"min_quote_amount_out"`
	BaseAmountOut     *string `json:"base_amount_out"`
	MaxQuoteAmountIn  *string `json:"max_quote_amount_in"`
	SpendableQuoteIn  *string `json:"spendable_quote_in"`
	MinBaseAmountOut  *string `json:"min_base_amount_out"`
}
