pub const BUY_IX_DISC: [u8; 8] = [102, 6, 61, 18, 1, 218, 235, 234];
pub const SELL_IX_DISC: [u8; 8] = [51, 230, 133, 164, 1, 127, 131, 173];
pub const BUY_EXACT_SOL_IN_IX_DISC: [u8; 8] = [56, 252, 116, 8, 158, 223, 205, 95];
pub const CREATE_IX_DISC: [u8; 8] = [24, 30, 200, 40, 5, 28, 7, 119];
pub const CREATE_V2_IX_DISC: [u8; 8] = [214, 144, 76, 236, 95, 139, 49, 180];
pub const MIGRATE_IX_DISC: [u8; 8] = [155, 234, 231, 146, 236, 158, 162, 30];

// Anchor event discriminator = sha256("event:<EventName>")[..8]
pub const TRADE_EVENT_DISC: [u8; 8] = [189, 219, 127, 211, 78, 230, 97, 238];
#[allow(dead_code)]
pub const CREATE_EVENT_DISC: [u8; 8] = [27, 114, 169, 77, 222, 235, 99, 118];
#[allow(dead_code)]
pub const CREATE_V2_EVENT_DISC: [u8; 8] = [90, 133, 138, 45, 185, 75, 7, 42];
#[allow(dead_code)]
pub const MIGRATE_EVENT_DISC: [u8; 8] = [189, 233, 93, 185, 92, 148, 234, 148];
#[allow(dead_code)]
pub const MIGRATE_BONDING_CURVE_CREATOR_EVENT_DISC: [u8; 8] =
    [155, 167, 104, 220, 213, 108, 243, 3];
