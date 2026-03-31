package events

type PumpfunTradePayload struct {
	Side                 string                      `json:"side"`
	IxName               string                      `json:"ix_name"`
	Mint                 string                      `json:"mint"`
	User                 string                      `json:"user"`
	BondingCurve         string                      `json:"bonding_curve"`
	Creator              string                      `json:"creator"`
	CreatorVault         string                      `json:"creator_vault"`
	TokenProgram         string                      `json:"token_program"`
	SolAmount            string                      `json:"sol_amount"`
	TokenAmount          string                      `json:"token_amount"`
	Fee                  string                      `json:"fee"`
	CreatorFee           string                      `json:"creator_fee"`
	VirtualSolReserves   string                      `json:"virtual_sol_reserves"`
	VirtualTokenReserves string                      `json:"virtual_token_reserves"`
	RealSolReserves      string                      `json:"real_sol_reserves"`
	RealTokenReserves    string                      `json:"real_token_reserves"`
	TrackVolume          bool                        `json:"track_volume"`
	MayhemMode           bool                        `json:"mayhem_mode"`
	Cashback             string                      `json:"cashback"`
	InstructionArgs      PumpfunTradeInstructionArgs `json:"instruction_args"`
}

type PumpfunTradeInstructionArgs struct {
	Amount         *string `json:"amount"`
	MaxSolCost     *string `json:"max_sol_cost"`
	MinSolOutput   *string `json:"min_sol_output"`
	SpendableSolIn *string `json:"spendable_sol_in"`
	MinTokensOut   *string `json:"min_tokens_out"`
}
