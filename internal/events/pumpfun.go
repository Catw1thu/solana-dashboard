package events

type PumpfunTradePayload struct {
	Side                   string                      `json:"side"`
	IxName                 string                      `json:"ix_name"`
	Mint                   string                      `json:"mint"`
	User                   string                      `json:"user"`
	BondingCurve           string                      `json:"bonding_curve"`
	AssociatedBondingCurve string                      `json:"associated_bonding_curve"`
	Creator                string                      `json:"creator"`
	CreatorVault           string                      `json:"creator_vault"`
	TokenProgram           string                      `json:"token_program"`
	SolAmount              string                      `json:"sol_amount"`
	TokenAmount            string                      `json:"token_amount"`
	Fee                    string                      `json:"fee"`
	CreatorFee             string                      `json:"creator_fee"`
	VirtualSolReserves     string                      `json:"virtual_sol_reserves"`
	VirtualTokenReserves   string                      `json:"virtual_token_reserves"`
	RealSolReserves        string                      `json:"real_sol_reserves"`
	RealTokenReserves      string                      `json:"real_token_reserves"`
	TrackVolume            bool                        `json:"track_volume"`
	MayhemMode             bool                        `json:"mayhem_mode"`
	Cashback               string                      `json:"cashback"`
	InstructionArgs        PumpfunTradeInstructionArgs `json:"instruction_args"`
}

type PumpfunTradeInstructionArgs struct {
	Amount         *string `json:"amount"`
	MaxSolCost     *string `json:"max_sol_cost"`
	MinSolOutput   *string `json:"min_sol_output"`
	SpendableSolIn *string `json:"spendable_sol_in"`
	MinTokensOut   *string `json:"min_tokens_out"`
}

type PumpfunCreatePayload struct {
	IxName               string  `json:"ix_name"`
	Mint                 string  `json:"mint"`
	BondingCurve         string  `json:"bonding_curve"`
	User                 string  `json:"user"`
	Creator              string  `json:"creator"`
	Name                 string  `json:"name"`
	Symbol               string  `json:"symbol"`
	URI                  string  `json:"uri"`
	TokenProgram         string  `json:"token_program"`
	VirtualTokenReserves string  `json:"virtual_token_reserves"`
	VirtualSolReserves   string  `json:"virtual_sol_reserves"`
	RealTokenReserves    string  `json:"real_token_reserves"`
	TokenTotalSupply     string  `json:"token_total_supply"`
	IsMayhemMode         bool    `json:"is_mayhem_mode"`
	IsCashbackEnabled    bool    `json:"is_cashback_enabled"`
	MintDecimals         *uint32 `json:"mint_decimals,omitempty"`
}

type PumpfunMigratePayload struct {
	Mint                   string `json:"mint"`
	User                   string `json:"user"`
	BondingCurve           string `json:"bonding_curve"`
	Pool                   string `json:"pool"`
	MintAmount             string `json:"mint_amount"`
	SolAmount              string `json:"sol_amount"`
	PoolMigrationFee       string `json:"pool_migration_fee"`
	WithdrawAuthority      string `json:"withdraw_authority"`
	AssociatedBondingCurve string `json:"associated_bonding_curve"`
	TokenProgram           string `json:"token_program"`
	PumpAmm                string `json:"pump_amm"`
	PoolAuthority          string `json:"pool_authority"`
	LPMint                 string `json:"lp_mint"`
}
