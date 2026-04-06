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

type PumpAmmCreatePoolPayload struct {
	Pool              string                           `json:"pool"`
	Creator           string                           `json:"creator"`
	BaseMint          string                           `json:"base_mint"`
	QuoteMint         string                           `json:"quote_mint"`
	LPMint            string                           `json:"lp_mint"`
	BaseAmountIn      string                           `json:"base_amount_in"`
	QuoteAmountIn     string                           `json:"quote_amount_in"`
	InitialLiquidity  string                           `json:"initial_liquidity"`
	CoinCreator       string                           `json:"coin_creator"`
	IsMayhemMode      bool                             `json:"is_mayhem_mode"`
	InstructionArgs   PumpAmmCreatePoolInstructionArgs `json:"instruction_args"`
	BaseMintDecimals  uint32                           `json:"base_mint_decimals"`
	QuoteMintDecimals uint32                           `json:"quote_mint_decimals"`
}

type PumpAmmCreatePoolInstructionArgs struct {
	Index          uint16 `json:"index"`
	CoinCreator    string `json:"coin_creator"`
	IsMayhemMode   bool   `json:"is_mayhem_mode"`
	IsCashbackCoin *bool  `json:"is_cashback_coin"`
}

type PumpAmmLiquidityPayload struct {
	Action           string                          `json:"action"`
	Pool             string                          `json:"pool"`
	User             string                          `json:"user"`
	BaseMint         string                          `json:"base_mint"`
	QuoteMint        string                          `json:"quote_mint"`
	LPMint           string                          `json:"lp_mint"`
	LPTokenAmountIn  *string                         `json:"lp_token_amount_in"`
	LPTokenAmountOut *string                         `json:"lp_token_amount_out"`
	BaseAmountIn     *string                         `json:"base_amount_in"`
	QuoteAmountIn    *string                         `json:"quote_amount_in"`
	BaseAmountOut    *string                         `json:"base_amount_out"`
	QuoteAmountOut   *string                         `json:"quote_amount_out"`
	LPMintSupply     string                          `json:"lp_mint_supply"`
	InstructionArgs  PumpAmmLiquidityInstructionArgs `json:"instruction_args"`
}

type PumpAmmLiquidityInstructionArgs struct {
	LPTokenAmountIn   *string `json:"lp_token_amount_in"`
	LPTokenAmountOut  *string `json:"lp_token_amount_out"`
	MaxBaseAmountIn   *string `json:"max_base_amount_in"`
	MaxQuoteAmountIn  *string `json:"max_quote_amount_in"`
	MinBaseAmountOut  *string `json:"min_base_amount_out"`
	MinQuoteAmountOut *string `json:"min_quote_amount_out"`
}
